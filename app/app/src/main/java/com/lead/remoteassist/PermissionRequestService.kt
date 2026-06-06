package com.lead.remoteassist

import android.app.Notification
import android.app.NotificationChannel
import android.app.NotificationManager
import android.app.Service
import android.content.Intent
import android.os.Handler
import android.os.IBinder
import android.os.Looper
import android.util.Log
import okhttp3.OkHttpClient
import okhttp3.Request
import okhttp3.Response
import okhttp3.WebSocket
import okhttp3.WebSocketListener
import org.json.JSONObject
import java.util.concurrent.TimeUnit

class PermissionRequestService : Service() {
    private val client = OkHttpClient.Builder()
        .pingInterval(15, TimeUnit.SECONDS)
        .build()
    private val handler = Handler(Looper.getMainLooper())
    private var webSocket: WebSocket? = null
    private var destroyed = false
    private var auth: ServerAuth? = null

    override fun onBind(intent: Intent?): IBinder? = null

    override fun onCreate() {
        super.onCreate()
        createNotificationChannel()
        startForeground(NOTIFICATION_ID, notification())
    }

    override fun onStartCommand(intent: Intent?, flags: Int, startId: Int): Int {
        destroyed = false
        if (!ScreenShareState.isSharing && webSocket == null) connectSignaling()
        return START_STICKY
    }

    override fun onDestroy() {
        destroyed = true
        handler.removeCallbacksAndMessages(null)
        webSocket?.close(1000, "permission service destroyed")
        webSocket = null
        client.dispatcher.executorService.shutdown()
        super.onDestroy()
    }

    private fun connectSignaling() {
        val saved = AuthStore.load(this)
        if (saved == null) {
            Log.i(TAG, "Not logged in; permission signaling disabled")
            return
        }
        val session = ServerAuth(token = saved.token, deviceId = saved.deviceId)
        auth = session
        openSignaling(saved.serverUrl, session)
    }

    private fun openSignaling(serverUrl: String, session: ServerAuth) {
        val wsUrl = AuthClient.authenticatedWebSocketUrl(serverUrl, session)
        webSocket = client.newWebSocket(
            Request.Builder().url(wsUrl).build(),
            object : WebSocketListener() {
                override fun onOpen(webSocket: WebSocket, response: Response) {
                    Log.i(TAG, "Permission signaling connected")
                    webSocket.send(
                        JSONObject()
                            .put("type", "hello")
                            .put("role", "android-waiting")
                            .put("deviceId", session.deviceId)
                            .toString(),
                    )
                }

                override fun onMessage(webSocket: WebSocket, text: String) {
                    val data = JSONObject(text)
                    if (data.optString("type") == "remote-request" && !ScreenShareState.isSharing) {
                        launchPermissionActivity()
                    }
                }

                override fun onClosed(webSocket: WebSocket, code: Int, reason: String) {
                    this@PermissionRequestService.webSocket = null
                    scheduleReconnect()
                }

                override fun onFailure(webSocket: WebSocket, t: Throwable, response: Response?) {
                    Log.e(TAG, "Permission signaling failed: ${t.message}", t)
                    this@PermissionRequestService.webSocket = null
                    scheduleReconnect()
                }
            },
        )
    }

    private fun scheduleReconnect() {
        if (destroyed || ScreenShareState.isSharing) return
        handler.postDelayed({
            if (!destroyed && !ScreenShareState.isSharing && webSocket == null) connectSignaling()
        }, RECONNECT_DELAY_MS)
    }

    private fun launchPermissionActivity() {
        val intent = Intent(this, MainActivity::class.java)
            .addFlags(Intent.FLAG_ACTIVITY_NEW_TASK or Intent.FLAG_ACTIVITY_SINGLE_TOP)
            .putExtra(MainActivity.EXTRA_AUTO_REQUEST_PERMISSION, true)
        startActivity(intent)
    }

    private fun notification(): Notification =
        Notification.Builder(this, CHANNEL_ID)
            .setContentTitle("远程协助")
            .setContentText("等待远程协助请求")
            .setSmallIcon(android.R.drawable.presence_invisible)
            .setOngoing(true)
            .build()

    private fun createNotificationChannel() {
        val manager = getSystemService(NotificationManager::class.java)
        manager.createNotificationChannel(
            NotificationChannel(CHANNEL_ID, "Permission Requests", NotificationManager.IMPORTANCE_LOW),
        )
    }

    companion object {
        private const val TAG = "PermissionRequestService"
        private const val CHANNEL_ID = "permission_requests"
        private const val NOTIFICATION_ID = 1002
        private const val RECONNECT_DELAY_MS = 1500L
    }
}
