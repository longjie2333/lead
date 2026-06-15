package com.lead.remoteassist

import android.accessibilityservice.AccessibilityService
import android.accessibilityservice.GestureDescription
import android.graphics.Path
import android.os.Handler
import android.os.Looper
import android.util.DisplayMetrics
import android.view.WindowManager
import android.view.accessibility.AccessibilityEvent
import kotlin.math.roundToInt

class RemoteControlAccessibilityService : AccessibilityService() {
    override fun onServiceConnected() {
        instance = this
    }

    override fun onDestroy() {
        if (instance === this) instance = null
        super.onDestroy()
    }

    override fun onAccessibilityEvent(event: AccessibilityEvent?) = Unit

    override fun onInterrupt() = Unit

    private fun tapNormalized(x: Double, y: Double, durationMs: Long): Boolean {
        val (px, py) = toScreenPoint(x, y)
        val path = Path().apply { moveTo(px, py) }
        return dispatch(path, durationMs.coerceIn(1L, 800L))
    }

    private fun swipeNormalized(
        startX: Double,
        startY: Double,
        endX: Double,
        endY: Double,
        durationMs: Long,
    ): Boolean {
        val (fromX, fromY) = toScreenPoint(startX, startY)
        val (toX, toY) = toScreenPoint(endX, endY)
        val path = Path().apply {
            moveTo(fromX, fromY)
            lineTo(toX, toY)
        }
        return dispatch(path, durationMs.coerceIn(60L, 1500L))
    }

    private fun multiSwipeNormalized(strokes: List<PointerStroke>, durationMs: Long): Boolean {
        if (strokes.isEmpty()) return false
        val builder = GestureDescription.Builder()
        val duration = durationMs.coerceIn(60L, 1500L)
        for (stroke in strokes.take(5)) {
            val (fromX, fromY) = toScreenPoint(stroke.startX, stroke.startY)
            val (toX, toY) = toScreenPoint(stroke.endX, stroke.endY)
            val path = Path().apply {
                moveTo(fromX, fromY)
                lineTo(toX, toY)
            }
            builder.addStroke(GestureDescription.StrokeDescription(path, 0L, duration))
        }
        return dispatchGesture(builder.build(), null, null)
    }

    private fun dispatch(path: Path, durationMs: Long): Boolean {
        val gesture = GestureDescription.Builder()
            .addStroke(GestureDescription.StrokeDescription(path, 0L, durationMs))
            .build()
        return dispatchGesture(gesture, null, null)
    }

    private fun toScreenPoint(x: Double, y: Double): Pair<Float, Float> {
        val metrics = currentDisplayMetrics()
        val px = (x.coerceIn(0.0, 1.0) * metrics.widthPixels).roundToInt()
        val py = (y.coerceIn(0.0, 1.0) * metrics.heightPixels).roundToInt()
        return px.toFloat() to py.toFloat()
    }

    private fun currentDisplayMetrics(): DisplayMetrics {
        val metrics = DisplayMetrics()
        if (android.os.Build.VERSION.SDK_INT >= android.os.Build.VERSION_CODES.R) {
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

    companion object {
        @Volatile
        private var instance: RemoteControlAccessibilityService? = null
        private val mainHandler = Handler(Looper.getMainLooper())

        fun isRunning(): Boolean = instance != null

        fun disableRemoteControl(callback: (Boolean) -> Unit) {
            mainHandler.post {
                val service = instance
                if (service == null) {
                    callback(false)
                    return@post
                }
                service.disableSelf()
                if (instance === service) instance = null
                callback(true)
            }
        }

        fun perform(action: String, payload: Map<String, Double> = emptyMap(), durationMs: Long = 120L, callback: (Boolean) -> Unit) {
            mainHandler.post {
                val service = instance
                if (service == null) {
                    callback(false)
                    return@post
                }
                val ok = when (action) {
                    "back" -> service.performGlobalAction(GLOBAL_ACTION_BACK)
                    "home" -> service.performGlobalAction(GLOBAL_ACTION_HOME)
                    "recents" -> service.performGlobalAction(GLOBAL_ACTION_RECENTS)
                    "tap" -> service.tapNormalized(
                        payload["x"] ?: return@post callback(false),
                        payload["y"] ?: return@post callback(false),
                        durationMs,
                    )
                    "longPress" -> service.tapNormalized(
                        payload["x"] ?: return@post callback(false),
                        payload["y"] ?: return@post callback(false),
                        durationMs.coerceAtLeast(550L),
                    )
                    "swipe" -> service.swipeNormalized(
                        payload["startX"] ?: return@post callback(false),
                        payload["startY"] ?: return@post callback(false),
                        payload["endX"] ?: return@post callback(false),
                        payload["endY"] ?: return@post callback(false),
                        durationMs,
                    )
                    else -> false
                }
                callback(ok)
            }
        }

        fun performMultiSwipe(strokes: List<PointerStroke>, durationMs: Long, callback: (Boolean) -> Unit) {
            mainHandler.post {
                val service = instance
                if (service == null) {
                    callback(false)
                    return@post
                }
                callback(service.multiSwipeNormalized(strokes, durationMs))
            }
        }
    }

    data class PointerStroke(
        val startX: Double,
        val startY: Double,
        val endX: Double,
        val endY: Double,
    )
}
