package com.lead.remoteassist

import android.Manifest
import android.app.Activity
import android.app.ActivityManager
import android.content.BroadcastReceiver
import android.content.ComponentName
import android.content.Context
import android.content.Intent
import android.content.IntentFilter
import android.media.projection.MediaProjectionConfig
import android.media.projection.MediaProjectionManager
import android.os.Build
import android.os.Bundle
import android.provider.Settings
import androidx.activity.ComponentActivity
import androidx.activity.compose.rememberLauncherForActivityResult
import androidx.activity.compose.setContent
import androidx.activity.result.contract.ActivityResultContracts
import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.runtime.DisposableEffect
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableIntStateOf
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.unit.dp
import androidx.core.content.ContextCompat
import androidx.lifecycle.Lifecycle
import androidx.lifecycle.LifecycleEventObserver
import top.yukonga.miuix.kmp.basic.Button
import top.yukonga.miuix.kmp.basic.Text
import top.yukonga.miuix.kmp.theme.MiuixTheme

class MainActivity : ComponentActivity() {
    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        startPermissionRequestService()
        val projectionManager = getSystemService(MediaProjectionManager::class.java)

        setContent {
            var projectionGranted by remember { mutableStateOf(false) }
            var controlEnabled by remember { mutableStateOf(isRemoteControlEnabled()) }
            var pendingAutoRequest by remember { mutableIntStateOf(if (shouldAutoRequest(intent)) 1 else 0) }
            val notificationPermissionLauncher = rememberLauncherForActivityResult(
                contract = ActivityResultContracts.RequestPermission(),
            ) { }
            val projectionLauncher = rememberLauncherForActivityResult(
                contract = ActivityResultContracts.StartActivityForResult(),
            ) { result ->
                val data = result.data
                if (result.resultCode == Activity.RESULT_OK && data != null) {
                    ScreenShareState.isSharing = true
                    projectionGranted = true
                    startScreenShareService(result.resultCode, data)
                } else {
                    ScreenShareState.isSharing = false
                    projectionGranted = false
                    startPermissionRequestService()
                }
            }

            fun requestProjectionPermission() {
                if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.TIRAMISU) {
                    notificationPermissionLauncher.launch(Manifest.permission.POST_NOTIFICATIONS)
                }
                val captureIntent = if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.UPSIDE_DOWN_CAKE) {
                    projectionManager.createScreenCaptureIntent(
                        MediaProjectionConfig.createConfigForDefaultDisplay(),
                    )
                } else {
                    projectionManager.createScreenCaptureIntent()
                }
                projectionLauncher.launch(captureIntent)
            }

            DisposableEffect(Unit) {
                projectionGranted = isScreenShareServiceRunning()
                controlEnabled = isRemoteControlEnabled()
                val receiver = object : BroadcastReceiver() {
                    override fun onReceive(context: Context, intent: Intent) {
                        if (intent.action == ScreenShareService.ACTION_SHARE_STOPPED) {
                            ScreenShareState.isSharing = false
                            projectionGranted = false
                            startPermissionRequestService()
                        }
                    }
                }
                ContextCompat.registerReceiver(
                    this@MainActivity,
                    receiver,
                    IntentFilter(ScreenShareService.ACTION_SHARE_STOPPED),
                    ContextCompat.RECEIVER_NOT_EXPORTED,
                )
                val lifecycleObserver = LifecycleEventObserver { _, event ->
                    if (event == Lifecycle.Event.ON_RESUME) {
                        projectionGranted = isScreenShareServiceRunning()
                        controlEnabled = isRemoteControlEnabled()
                    }
                }
                lifecycle.addObserver(lifecycleObserver)
                onDispose {
                    lifecycle.removeObserver(lifecycleObserver)
                    unregisterReceiver(receiver)
                }
            }

            LaunchedEffect(pendingAutoRequest) {
                if (pendingAutoRequest > 0 && !projectionGranted) {
                    requestProjectionPermission()
                }
            }

            MiuixTheme {
                Box(
                    modifier = Modifier
                        .fillMaxSize()
                        .background(Color(0xFFF7F8FA))
                        .padding(24.dp),
                    contentAlignment = Alignment.Center,
                ) {
                    Column(horizontalAlignment = Alignment.CenterHorizontally) {
                        if (projectionGranted) {
                            Text(text = "已获取权限，等待远程协助")
                        } else {
                            Button(
                                onClick = { requestProjectionPermission() },
                            ) {
                                Text(text = "立即开始")
                            }
                        }

                        if (!controlEnabled) {
                            Spacer(modifier = Modifier.height(16.dp))
                            Button(
                                onClick = {
                                    startActivity(Intent(Settings.ACTION_ACCESSIBILITY_SETTINGS))
                                },
                            ) {
                                Text(text = "启用远程控制")
                            }
                        }
                    }
                }
            }
        }
    }

    override fun onNewIntent(intent: Intent) {
        super.onNewIntent(intent)
        setIntent(intent)
        if (shouldAutoRequest(intent)) recreate()
    }

    private fun startScreenShareService(resultCode: Int, data: Intent) {
        stopService(Intent(this, PermissionRequestService::class.java))
        val serviceIntent = Intent(this, ScreenShareService::class.java)
            .putExtra(ScreenShareService.EXTRA_RESULT_CODE, resultCode)
            .putExtra(ScreenShareService.EXTRA_RESULT_DATA, data)
        ContextCompat.startForegroundService(this, serviceIntent)
    }

    private fun startPermissionRequestService() {
        ContextCompat.startForegroundService(this, Intent(this, PermissionRequestService::class.java))
    }

    private fun shouldAutoRequest(intent: Intent?): Boolean =
        intent?.getBooleanExtra(EXTRA_AUTO_REQUEST_PERMISSION, false) == true

    private fun isScreenShareServiceRunning(): Boolean {
        val manager = getSystemService(ActivityManager::class.java)
        @Suppress("DEPRECATION")
        return manager.getRunningServices(Int.MAX_VALUE).any {
            it.service.className == ScreenShareService::class.java.name
        }
    }

    private fun isRemoteControlEnabled(): Boolean {
        val expected = ComponentName(this, RemoteControlAccessibilityService::class.java).flattenToString()
        val enabledServices = Settings.Secure.getString(
            contentResolver,
            Settings.Secure.ENABLED_ACCESSIBILITY_SERVICES,
        ) ?: return false
        return enabledServices.split(':').any { it.equals(expected, ignoreCase = true) }
    }

    companion object {
        const val EXTRA_AUTO_REQUEST_PERMISSION = "extra_auto_request_permission"
    }
}
