package com.lead.remoteassist

import android.os.Build
import okhttp3.Call
import okhttp3.Callback
import okhttp3.MediaType.Companion.toMediaType
import okhttp3.OkHttpClient
import okhttp3.Request
import okhttp3.RequestBody.Companion.toRequestBody
import okhttp3.Response
import org.json.JSONObject
import java.io.IOException
import java.net.URLEncoder
import java.nio.charset.StandardCharsets

data class ServerAuth(
    val token: String,
    val deviceId: String,
)

object AuthClient {
    fun loginAndroid(
        client: OkHttpClient,
        serverUrl: String,
        username: String,
        password: String,
        deviceId: String,
        onResult: (Result<ServerAuth>) -> Unit,
    ) {
        val wsUrl = normalizeWebSocketUrl(serverUrl)
        val body = JSONObject()
            .put("username", username)
            .put("password", password)
            .put("source", "android")
            .put("deviceId", deviceId)
            .put("deviceName", Build.MODEL ?: deviceId)
            .toString()
            .toRequestBody("application/json; charset=utf-8".toMediaType())
        val request = try {
            Request.Builder()
                .url("${httpBaseUrl(wsUrl)}/api/login")
                .post(body)
                .build()
        } catch (error: IllegalArgumentException) {
            onResult(Result.failure(error))
            return
        }
        client.newCall(request).enqueue(object : Callback {
            override fun onFailure(call: Call, e: IOException) {
                onResult(Result.failure(e))
            }

            override fun onResponse(call: Call, response: Response) {
                response.use {
                    if (!it.isSuccessful) {
                        onResult(Result.failure(IOException("login failed: ${it.code}")))
                        return
                    }
                    val json = JSONObject(it.body.string())
                    val device = json.optJSONObject("device")
                    onResult(
                        Result.success(
                            ServerAuth(
                                token = json.getString("token"),
                                deviceId = device?.optString("id")?.takeIf { id -> id.isNotBlank() } ?: deviceId,
                            ),
                        ),
                    )
                }
            }
        })
    }

    fun authenticatedWebSocketUrl(wsUrl: String, auth: ServerAuth): String {
        val separator = if (wsUrl.contains("?")) "&" else "?"
        return "$wsUrl${separator}token=${urlEncode(auth.token)}&deviceId=${urlEncode(auth.deviceId)}"
    }

    fun normalizeWebSocketUrl(value: String): String {
        val input = value.trim().ifBlank { BuildConfig.SCREEN_SHARE_SERVER_URL }
        val withScheme = when {
            input.startsWith("http://") -> "ws://${input.removePrefix("http://")}"
            input.startsWith("https://") -> "wss://${input.removePrefix("https://")}"
            input.startsWith("ws://") || input.startsWith("wss://") -> input
            else -> "ws://$input"
        }
        return if (withScheme.endsWith("/ws")) withScheme else withScheme.trimEnd('/') + "/ws"
    }

    private fun httpBaseUrl(serverUrl: String): String {
        val noPath = normalizeWebSocketUrl(serverUrl).substringBeforeLast("/ws")
        return when {
            noPath.startsWith("wss://") -> "https://${noPath.removePrefix("wss://")}"
            noPath.startsWith("ws://") -> "http://${noPath.removePrefix("ws://")}"
            else -> noPath
        }
    }

    private fun urlEncode(value: String): String =
        URLEncoder.encode(value, StandardCharsets.UTF_8.name())
}
