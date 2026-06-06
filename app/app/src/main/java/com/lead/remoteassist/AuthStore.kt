package com.lead.remoteassist

import android.content.Context

data class SavedAuth(
    val serverUrl: String,
    val username: String,
    val token: String,
    val deviceId: String,
)

object AuthStore {
    private const val PREFS = "lead_auth"
    private const val KEY_SERVER_URL = "server_url"
    private const val KEY_USERNAME = "username"
    private const val KEY_TOKEN = "token"
    private const val KEY_DEVICE_ID = "device_id"

    fun load(context: Context): SavedAuth? {
        val prefs = context.getSharedPreferences(PREFS, Context.MODE_PRIVATE)
        val serverUrl = prefs.getString(KEY_SERVER_URL, "")?.trim().orEmpty()
        val username = prefs.getString(KEY_USERNAME, "")?.trim().orEmpty()
        val token = prefs.getString(KEY_TOKEN, "")?.trim().orEmpty()
        val deviceId = prefs.getString(KEY_DEVICE_ID, "")?.trim().orEmpty()
        if (serverUrl.isBlank() || username.isBlank() || token.isBlank() || deviceId.isBlank()) return null
        return SavedAuth(serverUrl, username, token, deviceId)
    }

    fun save(context: Context, serverUrl: String, username: String, auth: ServerAuth) {
        context.getSharedPreferences(PREFS, Context.MODE_PRIVATE)
            .edit()
            .putString(KEY_SERVER_URL, AuthClient.normalizeWebSocketUrl(serverUrl))
            .putString(KEY_USERNAME, username.trim())
            .putString(KEY_TOKEN, auth.token)
            .putString(KEY_DEVICE_ID, auth.deviceId)
            .apply()
    }

    fun clear(context: Context) {
        context.getSharedPreferences(PREFS, Context.MODE_PRIVATE).edit().clear().apply()
    }

    fun serverUrl(context: Context): String =
        context.getSharedPreferences(PREFS, Context.MODE_PRIVATE)
            .getString(KEY_SERVER_URL, BuildConfig.SCREEN_SHARE_SERVER_URL)
            ?.takeIf { it.isNotBlank() }
            ?: BuildConfig.SCREEN_SHARE_SERVER_URL

    fun username(context: Context): String =
        context.getSharedPreferences(PREFS, Context.MODE_PRIVATE)
            .getString(KEY_USERNAME, "")
            .orEmpty()
}
