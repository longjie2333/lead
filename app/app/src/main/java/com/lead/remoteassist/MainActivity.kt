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
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.text.KeyboardOptions
import androidx.compose.runtime.Composable
import androidx.compose.runtime.CompositionLocalProvider
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
import androidx.compose.ui.text.input.KeyboardType
import androidx.compose.ui.text.input.PasswordVisualTransformation
import androidx.compose.ui.text.input.VisualTransformation
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import androidx.core.content.ContextCompat
import androidx.lifecycle.Lifecycle
import androidx.lifecycle.LifecycleEventObserver
import androidx.navigationevent.compose.LocalNavigationEventDispatcherOwner
import androidx.navigationevent.compose.rememberNavigationEventDispatcherOwner
import okhttp3.OkHttpClient
import top.yukonga.miuix.kmp.basic.Button
import top.yukonga.miuix.kmp.basic.ButtonDefaults
import top.yukonga.miuix.kmp.basic.Icon
import top.yukonga.miuix.kmp.basic.Scaffold
import top.yukonga.miuix.kmp.basic.SmallTopAppBar
import top.yukonga.miuix.kmp.basic.Text
import top.yukonga.miuix.kmp.basic.TextField
import top.yukonga.miuix.kmp.extra.WindowDialog
import top.yukonga.miuix.kmp.icon.MiuixIcons
import top.yukonga.miuix.kmp.icon.extended.Play
import top.yukonga.miuix.kmp.icon.extended.RemoveContact
import top.yukonga.miuix.kmp.icon.extended.Replace
import top.yukonga.miuix.kmp.theme.MiuixTheme
import java.util.concurrent.TimeUnit

class MainActivity : ComponentActivity() {
    private val authClient = OkHttpClient.Builder()
        .connectTimeout(8, TimeUnit.SECONDS)
        .readTimeout(12, TimeUnit.SECONDS)
        .build()

    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        if (AuthStore.load(this) != null) startPermissionRequestService()
        val projectionManager = getSystemService(MediaProjectionManager::class.java)

        setContent {
            var savedAuth by remember { mutableStateOf(AuthStore.load(this)) }
            var username by remember { mutableStateOf(AuthStore.username(this).ifBlank { "admin" }) }
            var password by remember { mutableStateOf("") }
            var dialogMessage by remember { mutableStateOf("") }
            var isLoggingIn by remember { mutableStateOf(false) }
            var projectionGranted by remember { mutableStateOf(false) }
            var controlEnabled by remember { mutableStateOf(isRemoteControlEnabled()) }
            var pendingAutoRequest by remember { mutableIntStateOf(if (shouldAutoRequest(intent)) 1 else 0) }
            val navigationEventOwner = rememberNavigationEventDispatcherOwner(parent = null)
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
                    if (savedAuth != null) startPermissionRequestService()
                }
            }

            fun requestProjectionPermission() {
                if (savedAuth == null) {
                    dialogMessage = "请先登录账号"
                    return
                }
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

            fun login() {
                if (isLoggingIn) return
                if (username.isBlank() || password.isBlank()) {
                    dialogMessage = "请填写账号和密码"
                    return
                }
                isLoggingIn = true
                AuthClient.loginAndroid(
                    client = authClient,
                    serverUrl = BuildConfig.SCREEN_SHARE_SERVER_URL,
                    username = username,
                    password = password,
                    deviceId = localDeviceId(),
                ) { result ->
                    runOnUiThread {
                        isLoggingIn = false
                        result.onSuccess { auth ->
                            AuthStore.save(this, BuildConfig.SCREEN_SHARE_SERVER_URL, username, auth)
                            savedAuth = AuthStore.load(this)
                            password = ""
                            dialogMessage = ""
                            startPermissionRequestService()
                        }.onFailure {
                            dialogMessage = "登录失败：${it.message ?: "请检查账号密码或网络连接"}"
                        }
                    }
                }
            }

            fun logout() {
                AuthStore.clear(this)
                savedAuth = null
                dialogMessage = "已退出登录"
                stopService(Intent(this, PermissionRequestService::class.java))
                stopService(Intent(this, ScreenShareService::class.java))
            }

            DisposableEffect(Unit) {
                projectionGranted = isScreenShareServiceRunning()
                controlEnabled = isRemoteControlEnabled()
                val receiver = object : BroadcastReceiver() {
                    override fun onReceive(context: Context, intent: Intent) {
                        if (intent.action == ScreenShareService.ACTION_SHARE_STOPPED) {
                            ScreenShareState.isSharing = false
                            projectionGranted = false
                            if (savedAuth != null) startPermissionRequestService()
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

            LaunchedEffect(pendingAutoRequest, savedAuth) {
                if (pendingAutoRequest > 0 && savedAuth != null && !projectionGranted) {
                    requestProjectionPermission()
                }
            }

            CompositionLocalProvider(LocalNavigationEventDispatcherOwner provides navigationEventOwner) {

                if (savedAuth == null) Box(
                    modifier = Modifier
                        .fillMaxSize()
                        .background(Color(0xFFF7F8FA))
                        .padding(24.dp),
                    contentAlignment = Alignment.Center,
                ) {
                    LoginPanel(
                        username = username,
                        password = password,
                        isLoggingIn = isLoggingIn,
                        onUsernameChange = { username = it },
                        onPasswordChange = { password = it },
                        onLogin = { login() },
                    )
                }
                else Scaffold(
                    topBar = {
                        SmallTopAppBar(
                            title = "",
                            actions = {
                                Button(
                                    colors = ButtonDefaults.buttonColors(
                                        color = MiuixTheme.colorScheme.background
                                    ),
                                    insideMargin = PaddingValues(16.dp, 10.dp),
                                    onClick = { logout() }
                                ) {
                                    Column(
                                        horizontalAlignment = Alignment.End
                                    ) {
                                        Text(text = savedAuth?.username ?: username)
                                        Text(
                                            text = savedAuth?.deviceId ?: localDeviceId(),
                                            color = MiuixTheme.colorScheme.onSecondaryVariant,
                                            fontSize = 10.sp
                                        )
                                    }
                                    Spacer(modifier = Modifier.width(12.dp))
                                    Icon(
                                        MiuixIcons.RemoveContact,
                                        "account"
                                    )
                                }
                                Spacer(modifier = Modifier.width(10.dp))
                            }
                        )
                    },
                    content = {
                        Box(
                            modifier = Modifier
                                .fillMaxSize()
                                .background(MiuixTheme.colorScheme.surface)
                                .padding(24.dp),
                            contentAlignment = Alignment.Center,
                        ) {
                            Column(
                                modifier = Modifier.fillMaxWidth(),
                                horizontalAlignment = Alignment.CenterHorizontally,
                                verticalArrangement = Arrangement.Center,
                            ) {
                                Button(
                                    colors = ButtonDefaults.buttonColorsPrimary(),
                                    minWidth = 200.dp,
                                    minHeight = 120.dp,
                                    onClick = { requestProjectionPermission() }
                                ) {
                                    Column(
                                        horizontalAlignment = Alignment.CenterHorizontally
                                    ) {
                                        if (projectionGranted) {
                                            Icon(
                                                imageVector = MiuixIcons.Replace,
                                                contentDescription = "ReStart",
                                                modifier = Modifier.size(32.dp)
                                            )
                                            Spacer(Modifier.height(10.dp))
                                            Text(
                                                text = "重新授权协助",
                                                fontSize = 20.sp
                                            )
                                        } else {
                                            Icon(
                                                imageVector = MiuixIcons.Play,
                                                contentDescription = "Start",
                                                modifier = Modifier.size(32.dp)
                                            )
                                            Spacer(Modifier.height(10.dp))
                                            Text(
                                                text = "立即开始",
                                                fontSize = 20.sp
                                            )
                                        }
                                    }
                                }
                                Spacer(modifier = Modifier.height(12.dp))
                                Button(
                                    minWidth = 200.dp,
                                    minHeight = 60.dp,
                                    enabled = projectionGranted,
                                    onClick = {
                                        startActivity(Intent(Settings.ACTION_ACCESSIBILITY_SETTINGS))
                                    },
                                ) {
                                    Text(text = if (controlEnabled) "已允许对方控制" else "允许控制")
                                }
                            }
                        }
                    }
                )

                WindowDialog(
                    show = dialogMessage.isNotBlank(),
                    title = "提示",
                    summary = dialogMessage,
                    onDismissRequest = { dialogMessage = "" },
                ) {
                    Button(
                        onClick = { dialogMessage = "" },
                        modifier = Modifier.fillMaxWidth(),
                    ) {
                        Text(text = "确定")
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

    private fun localDeviceId(): String =
        Settings.Secure.getString(contentResolver, Settings.Secure.ANDROID_ID)
            ?.takeIf { it.isNotBlank() }
            ?: Build.MODEL

    companion object {
        const val EXTRA_AUTO_REQUEST_PERMISSION = "extra_auto_request_permission"
    }
}

@Composable
private fun LoginPanel(
    username: String,
    password: String,
    isLoggingIn: Boolean,
    onUsernameChange: (String) -> Unit,
    onPasswordChange: (String) -> Unit,
    onLogin: () -> Unit,
) {
    Column(
        modifier = Modifier
            .fillMaxWidth()
            .padding(16.dp, 0.dp)
    ) {
        LoginInput(
            label = "账号",
            value = username,
            onValueChange = onUsernameChange
        )
        Spacer(modifier = Modifier.height(10.dp))
        LoginInput(
            label = "密码",
            value = password,
            onValueChange = onPasswordChange,
            password = true,
        )
        Spacer(modifier = Modifier.height(14.dp))
        Button(
            onClick = onLogin,
            colors = ButtonDefaults.buttonColorsPrimary(),
            modifier = Modifier.fillMaxWidth()
        ) {
            Text(text = if (isLoggingIn) "登录中" else "登录")
        }
    }
}

@Composable
private fun LoginInput(
    label: String,
    value: String,
    onValueChange: (String) -> Unit,
    password: Boolean = false,
) {
    Column(modifier = Modifier.fillMaxWidth()) {
        TextField(
            label = label,
            value = value,
            onValueChange = onValueChange,
            modifier = Modifier
                .fillMaxWidth(),
            singleLine = true,
            keyboardOptions = KeyboardOptions(
                keyboardType = if (password) KeyboardType.Password else KeyboardType.Text,
            ),
            visualTransformation = if (password) PasswordVisualTransformation() else VisualTransformation.None,
        )
    }
}