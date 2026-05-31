package com.lead.remoteassist

import android.app.Notification
import android.app.NotificationChannel
import android.app.NotificationManager
import android.app.PendingIntent
import android.app.Service
import android.content.ComponentName
import android.content.Intent
import android.content.pm.ServiceInfo
import android.media.projection.MediaProjection
import android.os.Build
import android.os.IBinder
import android.provider.Settings
import android.util.DisplayMetrics
import android.util.Log
import android.view.WindowManager
import okhttp3.OkHttpClient
import okhttp3.Request
import okhttp3.Response
import okhttp3.WebSocket
import okhttp3.WebSocketListener
import org.json.JSONArray
import org.json.JSONObject
import org.webrtc.CandidatePairChangeEvent
import org.webrtc.DataChannel
import org.webrtc.DefaultVideoDecoderFactory
import org.webrtc.DefaultVideoEncoderFactory
import org.webrtc.EglBase
import org.webrtc.IceCandidate
import org.webrtc.MediaConstraints
import org.webrtc.MediaStream
import org.webrtc.PeerConnection
import org.webrtc.PeerConnectionFactory
import org.webrtc.RtpReceiver
import org.webrtc.RtpSender
import org.webrtc.RtpTransceiver
import org.webrtc.ScreenCapturerAndroid
import org.webrtc.SdpObserver
import org.webrtc.SessionDescription
import org.webrtc.SurfaceTextureHelper
import org.webrtc.VideoSource
import org.webrtc.VideoTrack
import java.nio.ByteBuffer
import java.nio.charset.StandardCharsets
import java.util.concurrent.TimeUnit
import kotlin.math.max
import kotlin.math.roundToInt

class ScreenShareService : Service() {
    private val client = OkHttpClient.Builder()
        .pingInterval(15, TimeUnit.SECONDS)
        .build()
    private val peers = mutableMapOf<String, PeerConnection>()
    private val senders = mutableMapOf<String, RtpSender>()
    private val controlChannels = mutableMapOf<String, DataChannel>()
    private val iceServers = mutableListOf<PeerConnection.IceServer>()
    private var qualityProfile = QualityProfile.BALANCED

    private var webSocket: WebSocket? = null
    private var eglBase: EglBase? = null
    private var peerConnectionFactory: PeerConnectionFactory? = null
    private var surfaceTextureHelper: SurfaceTextureHelper? = null
    private var videoSource: VideoSource? = null
    private var videoTrack: VideoTrack? = null
    private var screenCapturer: ScreenCapturerAndroid? = null
    private var captureWidth = 720
    private var captureHeight = 1280

    override fun onBind(intent: Intent?): IBinder? = null

    override fun onCreate() {
        super.onCreate()
        createNotificationChannel()
    }

    override fun onStartCommand(intent: Intent?, flags: Int, startId: Int): Int {
        if (intent?.action == ACTION_STOP_SHARE) {
            stopSelf()
            return START_NOT_STICKY
        }

        startForegroundCompat()
        if (intent == null) return START_STICKY

        val resultData = if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.TIRAMISU) {
            intent.getParcelableExtra(EXTRA_RESULT_DATA, Intent::class.java)
        } else {
            @Suppress("DEPRECATION")
            intent.getParcelableExtra(EXTRA_RESULT_DATA)
        }
        if (resultData == null) {
            ScreenShareState.isSharing = false
            sendBroadcast(Intent(ACTION_SHARE_STOPPED).setPackage(packageName))
            stopSelf()
            return START_NOT_STICKY
        }

        ScreenShareState.isSharing = true
        startWebRtc(resultData)
        return START_STICKY
    }

    override fun onDestroy() {
        ScreenShareState.isSharing = false
        peers.values.forEach { it.close() }
        peers.clear()
        senders.clear()
        controlChannels.values.forEach { it.close() }
        controlChannels.clear()
        runCatching { screenCapturer?.stopCapture() }
        screenCapturer?.dispose()
        videoSource?.dispose()
        videoTrack?.dispose()
        surfaceTextureHelper?.dispose()
        peerConnectionFactory?.dispose()
        eglBase?.release()
        webSocket?.close(1000, "service destroyed")
        client.dispatcher.executorService.shutdown()
        sendBroadcast(Intent(ACTION_SHARE_STOPPED).setPackage(packageName))
        super.onDestroy()
    }

    private fun startWebRtc(resultData: Intent) {
        val metrics = currentDisplayMetrics()
        val (width, height) = targetSize(metrics.widthPixels, metrics.heightPixels, qualityProfile.maxLongEdge)
        captureWidth = width
        captureHeight = height

        PeerConnectionFactory.initialize(
            PeerConnectionFactory.InitializationOptions.builder(this)
                .setEnableInternalTracer(false)
                .createInitializationOptions(),
        )
        eglBase = EglBase.create()
        val encoderFactory = DefaultVideoEncoderFactory(eglBase!!.eglBaseContext, true, true)
        val decoderFactory = DefaultVideoDecoderFactory(eglBase!!.eglBaseContext)
        peerConnectionFactory = PeerConnectionFactory.builder()
            .setVideoEncoderFactory(encoderFactory)
            .setVideoDecoderFactory(decoderFactory)
            .createPeerConnectionFactory()

        surfaceTextureHelper = SurfaceTextureHelper.create("ScreenCaptureThread", eglBase!!.eglBaseContext)
        videoSource = peerConnectionFactory!!.createVideoSource(false)
        screenCapturer = ScreenCapturerAndroid(
            resultData,
            object : MediaProjection.Callback() {
                override fun onStop() {
                    Log.i(TAG, "MediaProjection stopped")
                    stopSelf()
                }
            },
        )
        screenCapturer!!.initialize(surfaceTextureHelper, this, videoSource!!.capturerObserver)
        screenCapturer!!.startCapture(captureWidth, captureHeight, qualityProfile.frameRate)
        videoTrack = peerConnectionFactory!!.createVideoTrack(VIDEO_TRACK_ID, videoSource)

        connectSignaling()
    }

    private fun connectSignaling() {
        webSocket = client.newWebSocket(
            Request.Builder().url(BuildConfig.SCREEN_SHARE_SERVER_URL).build(),
            object : WebSocketListener() {
                override fun onOpen(webSocket: WebSocket, response: Response) {
                    Log.i(TAG, "Signaling connected to ${BuildConfig.SCREEN_SHARE_SERVER_URL}")
                    sendJson(
                        JSONObject()
                            .put("type", "hello")
                            .put("role", "android")
                            .put("width", captureWidth)
                            .put("height", captureHeight)
                            .put("controlEnabled", isRemoteControlEnabled()),
                    )
                }

                override fun onMessage(webSocket: WebSocket, text: String) {
                    handleSignal(JSONObject(text))
                }

                override fun onFailure(webSocket: WebSocket, t: Throwable, response: Response?) {
                    Log.e(TAG, "Signaling failed: ${t.message}", t)
                }
            },
        )
    }

    private fun handleSignal(data: JSONObject) {
        when (data.optString("type")) {
            "config" -> updateIceServers(data.optJSONArray("iceServers"))
            "viewer-joined" -> createPeerForViewer(data.getString("viewerId"))
            "answer" -> peers[data.getString("viewerId")]?.setRemoteDescription(
                loggingSdpObserver("Set answer ${data.getString("viewerId")}"),
                SessionDescription(SessionDescription.Type.ANSWER, data.getString("sdp")),
            )
            "candidate" -> peers[data.getString("viewerId")]?.addIceCandidate(
                IceCandidate(
                    data.getString("sdpMid"),
                    data.getInt("sdpMLineIndex"),
                    data.getString("candidate"),
                ),
            )
            "renegotiate" -> {
                val viewerId = data.getString("viewerId")
                removePeer(viewerId)
                createPeerForViewer(viewerId)
            }
            "quality-mode" -> applyQualityProfile(QualityProfile.fromMode(data.optString("mode")))
            "control" -> handleRemoteControl(data, fromDataChannel = false)
            "control-permission-request" -> handleControlPermissionRequest(data.optString("viewerId"))
            "viewer-left" -> removePeer(data.getString("viewerId"))
        }
    }

    private fun handleControlPermissionRequest(viewerId: String) {
        val enabled = isRemoteControlEnabled()
        if (!enabled) {
            runCatching {
                startActivity(
                    Intent(Settings.ACTION_ACCESSIBILITY_SETTINGS)
                        .addFlags(Intent.FLAG_ACTIVITY_NEW_TASK),
                )
            }.onFailure {
                Log.e(TAG, "Open accessibility settings failed: ${it.message}", it)
            }
        }
        sendControlPermissionStatus(
            viewerId = viewerId,
            enabled = enabled,
            message = if (enabled) "控制权限已开启" else "已在安卓端打开无障碍设置，请启用 Lead Remote Assist",
        )
    }

    private fun handleRemoteControl(data: JSONObject, fromDataChannel: Boolean) {
        val viewerId = data.optString("viewerId")
        val action = data.optString("action")
        if (action == "pointer") return
        if (action == "multiSwipe") {
            val pointers = data.optJSONArray("pointers") ?: JSONArray()
            val strokes = mutableListOf<RemoteControlAccessibilityService.PointerStroke>()
            for (index in 0 until pointers.length()) {
                val pointer = pointers.optJSONObject(index) ?: continue
                strokes += RemoteControlAccessibilityService.PointerStroke(
                    startX = pointer.optDouble("startX"),
                    startY = pointer.optDouble("startY"),
                    endX = pointer.optDouble("endX"),
                    endY = pointer.optDouble("endY"),
                )
            }
            RemoteControlAccessibilityService.performMultiSwipe(strokes, data.optLong("durationMs", 180L)) { ok ->
                sendControlAck(viewerId, action, ok, fromDataChannel)
            }
            return
        }

        val payload = mutableMapOf<String, Double>()
        for (key in listOf("x", "y", "startX", "startY", "endX", "endY")) {
            if (data.has(key)) payload[key] = data.optDouble(key)
        }
        val durationMs = data.optLong("durationMs", 120L)

        RemoteControlAccessibilityService.perform(action, payload, durationMs) { ok ->
            sendControlAck(viewerId, action, ok, fromDataChannel)
        }
    }

    private fun sendControlAck(viewerId: String, action: String, ok: Boolean, preferDataChannel: Boolean) {
        val ack = JSONObject()
            .put("type", "control-ack")
            .put("viewerId", viewerId)
            .put("action", action)
            .put("ok", ok)
            .put("message", if (ok) "ok" else "需要在安卓端启用远程控制服务")
        if (preferDataChannel && sendDataChannel(viewerId, ack)) return
        sendJson(ack)
    }

    private fun createPeerForViewer(viewerId: String) {
        if (peers.containsKey(viewerId)) return
        val rtcConfig = PeerConnection.RTCConfiguration(iceServers.ifEmpty { defaultIceServers() }).apply {
            sdpSemantics = PeerConnection.SdpSemantics.UNIFIED_PLAN
            continualGatheringPolicy = PeerConnection.ContinualGatheringPolicy.GATHER_CONTINUALLY
            tcpCandidatePolicy = PeerConnection.TcpCandidatePolicy.ENABLED
        }
        val peerConnection = peerConnectionFactory?.createPeerConnection(rtcConfig, object : PeerConnection.Observer {
            override fun onIceCandidate(candidate: IceCandidate) {
                Log.i(TAG, "Peer $viewerId local ICE ${candidateType(candidate.sdp)}")
                sendJson(
                    JSONObject()
                        .put("type", "candidate")
                        .put("viewerId", viewerId)
                        .put("candidate", candidate.sdp)
                        .put("sdpMid", candidate.sdpMid)
                        .put("sdpMLineIndex", candidate.sdpMLineIndex),
                )
            }

            override fun onConnectionChange(newState: PeerConnection.PeerConnectionState) {
                Log.i(TAG, "Peer $viewerId state=$newState")
            }

            override fun onSelectedCandidatePairChanged(event: CandidatePairChangeEvent) {
                sendJson(
                    JSONObject()
                        .put("type", "route")
                        .put("viewerId", viewerId)
                        .put("localType", candidateType(event.local.sdp))
                        .put("remoteType", candidateType(event.remote.sdp)),
                )
            }

            override fun onSignalingChange(state: PeerConnection.SignalingState) = Unit
            override fun onIceConnectionChange(state: PeerConnection.IceConnectionState) = Unit
            override fun onIceConnectionReceivingChange(receiving: Boolean) = Unit
            override fun onIceGatheringChange(state: PeerConnection.IceGatheringState) {
                Log.i(TAG, "Peer $viewerId ICE gathering=$state")
            }
            override fun onIceCandidatesRemoved(candidates: Array<out IceCandidate>) = Unit
            override fun onAddStream(stream: MediaStream) = Unit
            override fun onRemoveStream(stream: MediaStream) = Unit
            override fun onDataChannel(channel: org.webrtc.DataChannel) = Unit
            override fun onRenegotiationNeeded() = Unit
            override fun onAddTrack(receiver: RtpReceiver, streams: Array<out MediaStream>) = Unit
            override fun onTrack(transceiver: RtpTransceiver) = Unit
        }) ?: return

        val track = videoTrack ?: return
        peers[viewerId] = peerConnection
        val sender = peerConnection.addTrack(track, listOf(STREAM_ID))
        senders[viewerId] = sender
        createControlChannel(viewerId, peerConnection)
        applySenderProfile(sender, qualityProfile)
        peerConnection.createOffer(object : SdpObserver {
            override fun onCreateSuccess(description: SessionDescription) {
                peerConnection.setLocalDescription(
                    object : SdpObserver {
                        override fun onSetSuccess() {
                            sendJson(
                                JSONObject()
                                    .put("type", "offer")
                                    .put("viewerId", viewerId)
                                    .put("sdp", description.description),
                            )
                        }

                        override fun onSetFailure(error: String) {
                            Log.e(TAG, "Offer set failed for $viewerId: $error")
                        }

                        override fun onCreateSuccess(description: SessionDescription) = Unit
                        override fun onCreateFailure(error: String) = Unit
                    },
                    description,
                )
            }

            override fun onSetSuccess() = Unit
            override fun onCreateFailure(error: String) {
                Log.e(TAG, "Offer failed: $error")
            }

            override fun onSetFailure(error: String) {
                Log.e(TAG, "Offer set failed: $error")
            }
        }, MediaConstraints())
    }

    private fun removePeer(viewerId: String) {
        senders.remove(viewerId)
        controlChannels.remove(viewerId)?.close()
        peers.remove(viewerId)?.close()
    }

    private fun createControlChannel(viewerId: String, peerConnection: PeerConnection) {
        val channel = peerConnection.createDataChannel("control", DataChannel.Init().apply {
            ordered = false
            maxRetransmits = 0
        }) ?: return
        controlChannels[viewerId] = channel
        channel.registerObserver(object : DataChannel.Observer {
            override fun onBufferedAmountChange(previousAmount: Long) = Unit
            override fun onStateChange() {
                Log.i(TAG, "Control DataChannel $viewerId state=${channel.state()}")
            }
            override fun onMessage(buffer: DataChannel.Buffer) {
                if (buffer.binary) return
                val bytes = ByteArray(buffer.data.remaining())
                buffer.data.get(bytes)
                runCatching {
                    val data = JSONObject(String(bytes, StandardCharsets.UTF_8)).put("viewerId", viewerId)
                    if (data.optString("type") == "control") handleRemoteControl(data, fromDataChannel = true)
                }.onFailure {
                    Log.e(TAG, "Invalid control DataChannel message: ${it.message}", it)
                }
            }
        })
    }

    private fun sendDataChannel(viewerId: String, data: JSONObject): Boolean {
        val channel = controlChannels[viewerId] ?: return false
        if (channel.state() != DataChannel.State.OPEN) return false
        val bytes = data.toString().toByteArray(StandardCharsets.UTF_8)
        return channel.send(DataChannel.Buffer(ByteBuffer.wrap(bytes), false))
    }

    private fun applyQualityProfile(profile: QualityProfile) {
        if (qualityProfile == profile) return
        qualityProfile = profile
        val metrics = currentDisplayMetrics()
        val (width, height) = targetSize(metrics.widthPixels, metrics.heightPixels, profile.maxLongEdge)
        captureWidth = width
        captureHeight = height
        runCatching {
            screenCapturer?.changeCaptureFormat(captureWidth, captureHeight, profile.frameRate)
        }.onFailure {
            Log.e(TAG, "Change capture format failed: ${it.message}", it)
        }
        senders.values.forEach { applySenderProfile(it, profile) }
        sendJson(
            JSONObject()
                .put("type", "stream-info")
                .put("width", captureWidth)
                .put("height", captureHeight)
                .put("qualityMode", profile.mode)
                .put("controlEnabled", isRemoteControlEnabled()),
        )
        Log.i(TAG, "Quality mode=${profile.mode} ${captureWidth}x$captureHeight ${profile.frameRate}fps ${profile.maxBitrateBps}bps")
    }

    private fun sendControlPermissionStatus(viewerId: String, enabled: Boolean, message: String) {
        sendJson(
            JSONObject()
                .put("type", "control-permission-status")
                .put("viewerId", viewerId)
                .put("enabled", enabled)
                .put("message", message),
        )
    }

    private fun isRemoteControlEnabled(): Boolean {
        val expected = ComponentName(this, RemoteControlAccessibilityService::class.java).flattenToString()
        val enabledServices = Settings.Secure.getString(
            contentResolver,
            Settings.Secure.ENABLED_ACCESSIBILITY_SERVICES,
        ) ?: return false
        return enabledServices.split(':').any { it.equals(expected, ignoreCase = true) }
    }

    private fun applySenderProfile(sender: RtpSender, profile: QualityProfile) {
        val parameters = sender.parameters ?: return
        parameters.degradationPreference = profile.degradationPreference
        for (encoding in parameters.encodings) {
            encoding.maxBitrateBps = profile.maxBitrateBps
            encoding.minBitrateBps = profile.minBitrateBps
            encoding.maxFramerate = profile.frameRate
            encoding.scaleResolutionDownBy = 1.0
        }
        if (!sender.setParameters(parameters)) {
            Log.w(TAG, "Failed to apply RTP parameters for ${profile.mode}")
        }
    }

    private fun updateIceServers(servers: JSONArray?) {
        iceServers.clear()
        if (servers == null) {
            iceServers.addAll(defaultIceServers())
            return
        }
        for (index in 0 until servers.length()) {
            val server = servers.getJSONObject(index)
            val urls = server.get("urls")
            val builder = when (urls) {
                is JSONArray -> PeerConnection.IceServer.builder((0 until urls.length()).map { urls.getString(it) })
                else -> PeerConnection.IceServer.builder(urls.toString())
            }
            val username = server.optString("username", "")
            val credential = server.optString("credential", "")
            if (username.isNotBlank() || credential.isNotBlank()) {
                builder.setUsername(username).setPassword(credential)
            }
            iceServers.add(builder.createIceServer())
        }
        if (iceServers.isEmpty()) iceServers.addAll(defaultIceServers())
    }

    private fun defaultIceServers(): List<PeerConnection.IceServer> = listOf(
        PeerConnection.IceServer.builder("stun:stun.l.google.com:19302").createIceServer(),
    )

    private fun sendJson(data: JSONObject) {
        webSocket?.send(data.toString())
    }

    private fun candidateType(candidate: String): String {
        val parts = candidate.split(" ")
        val typeIndex = parts.indexOf("typ")
        return if (typeIndex >= 0 && typeIndex + 1 < parts.size) parts[typeIndex + 1] else "--"
    }

    private fun loggingSdpObserver(operation: String): SdpObserver = object : SdpObserver {
        override fun onCreateSuccess(description: SessionDescription) = Unit
        override fun onSetSuccess() {
            Log.i(TAG, "$operation succeeded")
        }

        override fun onCreateFailure(error: String) {
            Log.e(TAG, "$operation create failed: $error")
        }

        override fun onSetFailure(error: String) {
            Log.e(TAG, "$operation set failed: $error")
        }
    }

    private fun currentDisplayMetrics(): DisplayMetrics {
        val metrics = DisplayMetrics()
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.R) {
            val bounds = getSystemService(WindowManager::class.java).currentWindowMetrics.bounds
            metrics.widthPixels = bounds.width()
            metrics.heightPixels = bounds.height()
            metrics.densityDpi = resources.displayMetrics.densityDpi
        } else {
            @Suppress("DEPRECATION")
            getSystemService(WindowManager::class.java).defaultDisplay.getRealMetrics(metrics)
        }
        return metrics
    }

    private fun even(value: Int): Int = if (value % 2 == 0) value else value - 1

    private fun targetSize(sourceWidth: Int, sourceHeight: Int, maxLongEdge: Int): Pair<Int, Int> {
        val longEdge = max(sourceWidth, sourceHeight)
        val scale = minOf(1f, maxLongEdge.toFloat() / longEdge.toFloat())
        return even(max(2, (sourceWidth * scale).roundToInt())) to
            even(max(2, (sourceHeight * scale).roundToInt()))
    }

    private fun startForegroundCompat() {
        val stopIntent = PendingIntent.getService(
            this,
            0,
            Intent(this, ScreenShareService::class.java).setAction(ACTION_STOP_SHARE),
            PendingIntent.FLAG_IMMUTABLE or PendingIntent.FLAG_UPDATE_CURRENT,
        )
        val notification = Notification.Builder(this, CHANNEL_ID)
            .setContentTitle("远程协助")
            .setContentText("正在共享屏幕")
            .setSmallIcon(android.R.drawable.presence_video_online)
            .addAction(android.R.drawable.ic_menu_close_clear_cancel, "停止共享", stopIntent)
            .setOngoing(true)
            .build()
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.Q) {
            startForeground(NOTIFICATION_ID, notification, ServiceInfo.FOREGROUND_SERVICE_TYPE_MEDIA_PROJECTION)
        } else {
            startForeground(NOTIFICATION_ID, notification)
        }
    }

    private fun createNotificationChannel() {
        val manager = getSystemService(NotificationManager::class.java)
        manager.createNotificationChannel(
            NotificationChannel(CHANNEL_ID, "Screen Share", NotificationManager.IMPORTANCE_LOW),
        )
    }

    private data class QualityProfile(
        val mode: String,
        val maxLongEdge: Int,
        val frameRate: Int,
        val minBitrateBps: Int,
        val maxBitrateBps: Int,
        val degradationPreference: org.webrtc.RtpParameters.DegradationPreference,
    ) {
        companion object {
            val QUALITY = QualityProfile(
                mode = "quality",
                maxLongEdge = 1600,
                frameRate = 30,
                minBitrateBps = 1_200_000,
                maxBitrateBps = 3_000_000,
                degradationPreference = org.webrtc.RtpParameters.DegradationPreference.MAINTAIN_RESOLUTION,
            )
            val BALANCED = QualityProfile(
                mode = "balanced",
                maxLongEdge = 1280,
                frameRate = 24,
                minBitrateBps = 800_000,
                maxBitrateBps = 2_000_000,
                degradationPreference = org.webrtc.RtpParameters.DegradationPreference.BALANCED,
            )
            val SPEED = QualityProfile(
                mode = "speed",
                maxLongEdge = 960,
                frameRate = 15,
                minBitrateBps = 300_000,
                maxBitrateBps = 900_000,
                degradationPreference = org.webrtc.RtpParameters.DegradationPreference.MAINTAIN_FRAMERATE,
            )

            fun fromMode(mode: String): QualityProfile = when (mode) {
                "quality" -> QUALITY
                "speed" -> SPEED
                else -> BALANCED
            }
        }
    }

    companion object {
        private const val TAG = "ScreenShareService"
        const val EXTRA_RESULT_CODE = "extra_result_code"
        const val EXTRA_RESULT_DATA = "extra_result_data"
        const val ACTION_SHARE_STOPPED = "com.lead.remoteassist.SHARE_STOPPED"
        private const val ACTION_STOP_SHARE = "com.lead.remoteassist.STOP_SHARE"
        private const val CHANNEL_ID = "screen_share"
        private const val NOTIFICATION_ID = 1001
        private const val STREAM_ID = "screen-stream"
        private const val VIDEO_TRACK_ID = "screen-video"
    }
}

