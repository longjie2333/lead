plugins {
    id("com.android.application")
    id("org.jetbrains.kotlin.android")
    id("org.jetbrains.kotlin.plugin.compose")
}

android {
    namespace = "com.lead.remoteassist"
    compileSdk = 36

    defaultConfig {
        applicationId = "com.lead.remoteassist"
        minSdk = 26
        targetSdk = 36
        versionCode = 1
        versionName = "1.0"

        val defaultServerUrl = providers.gradleProperty("screenShareServerUrl")
            .getOrElse("ws://10.0.2.2:8787/ws")
        buildConfigField("String", "SCREEN_SHARE_SERVER_URL", "\"$defaultServerUrl\"")
    }

    buildFeatures {
        compose = true
        buildConfig = true
    }

    kotlin {
        jvmToolchain(17)
    }
}

dependencies {
    implementation("androidx.activity:activity-compose:1.11.0")
    implementation("androidx.core:core-ktx:1.17.0")
    implementation("androidx.lifecycle:lifecycle-runtime-ktx:2.10.0")
    implementation("androidx.navigationevent:navigationevent-compose-android:1.0.2")
    implementation("org.jetbrains.compose.foundation:foundation:1.10.3")
    implementation("top.yukonga.miuix.kmp:miuix-android:0.8.8")
    implementation("com.squareup.okhttp3:okhttp:5.3.2")
    implementation("io.github.webrtc-sdk:android:144.7559.05")
}
