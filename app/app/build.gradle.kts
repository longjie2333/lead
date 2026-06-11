import java.util.Properties

plugins {
    id("com.android.application")
    id("org.jetbrains.kotlin.android")
    id("org.jetbrains.kotlin.plugin.compose")
}

val signingPropertiesFile = rootProject.file("signing.properties")
val signingProperties = Properties()

if (signingPropertiesFile.exists()) {
    signingPropertiesFile.inputStream().use(signingProperties::load)
}

android {
    namespace = "com.lead.remoteassist"
    compileSdk = 36

    defaultConfig {
        applicationId = "com.lead.remoteassist"
        minSdk = 26
        targetSdk = 36
        versionCode = providers.gradleProperty("versionCode")
            .orElse(providers.environmentVariable("VERSION_CODE"))
            .getOrElse("1")
            .toInt()
        versionName = providers.gradleProperty("versionName")
            .orElse(providers.environmentVariable("VERSION_NAME"))
            .getOrElse("1.0")

        val defaultServerUrl = providers.gradleProperty("screenShareServerUrl")
            .orElse(providers.environmentVariable("SCREEN_SHARE_SERVER_URL"))
            .getOrElse("ws://10.0.2.2:8787/ws")
        buildConfigField("String", "SCREEN_SHARE_SERVER_URL", "\"$defaultServerUrl\"")
    }

    signingConfigs {
        create("release") {
            val storeFilePath = signingProperties.getProperty("storeFile")

            if (!storeFilePath.isNullOrBlank()) {
                storeFile = file(storeFilePath)
            }

            storePassword = signingProperties.getProperty("storePassword")
            keyAlias = signingProperties.getProperty("keyAlias")
            keyPassword = signingProperties.getProperty("keyPassword")
        }
    }

    buildTypes {
        release {
            if (signingPropertiesFile.exists()) {
                signingConfig = signingConfigs.getByName("release")
            }

            isMinifyEnabled = true
            isShrinkResources = true
        }
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
    implementation("top.yukonga.miuix.kmp:miuix-icons:0.8.8")
    implementation("com.squareup.okhttp3:okhttp:5.3.2")
    implementation("io.github.webrtc-sdk:android:144.7559.05")
}
