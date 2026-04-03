package com.pocketserver.app

import android.app.ActivityManager
import android.content.Context
import android.content.Intent
import android.net.Uri
import android.os.Build
import android.os.PowerManager
import android.provider.Settings
import androidx.annotation.NonNull
import io.flutter.embedding.android.FlutterActivity
import io.flutter.embedding.engine.FlutterEngine
import io.flutter.plugin.common.MethodChannel
import android.os.MemoryInfo

class MainActivity: FlutterActivity() {
    private val CHANNEL = "com.pocketserver.app/battery"
    private val MEMORY_CHANNEL = "com.pocketserver.app/memory"

    override fun configureFlutterEngine(@NonNull flutterEngine: FlutterEngine) {
        super.configureFlutterEngine(flutterEngine)
        
        // Battery optimization channel
        MethodChannel(flutterEngine.dartExecutor.binaryMessenger, CHANNEL).setMethodCallHandler { call, result ->
            when (call.method) {
                "isBatteryOptimizationDisabled" -> {
                    val isDisabled = isBatteryOptimizationDisabled()
                    result.success(isDisabled)
                }
                "requestBatteryOptimizationExemption" -> {
                    requestBatteryOptimizationExemption()
                    result.success(null)
                }
                else -> {
                    result.notImplemented()
                }
            }
        }

        // Memory pressure channel
        MethodChannel(flutterEngine.dartExecutor.binaryMessenger, MEMORY_CHANNEL).setMethodCallHandler { call, result ->
            when (call.method) {
                "getMemoryPressureLevel" -> {
                    val level = getMemoryPressureLevel()
                    result.success(level)
                }
                else -> {
                    result.notImplemented()
                }
            }
        }
    }

    /**
     * Checks if battery optimization is disabled for this app.
     * Returns true if the app can run in background without restrictions.
     */
    private fun isBatteryOptimizationDisabled(): Boolean {
        return if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.M) {
            val powerManager = getSystemService(Context.POWER_SERVICE) as PowerManager
            powerManager.isIgnoringBatteryOptimizations(packageName)
        } else {
            // Pre-Marshmallow devices don't have battery optimization
            true
        }
    }

    /**
     * Opens system settings to request battery optimization exemption.
     * The user must manually disable optimization for this app.
     */
    private fun requestBatteryOptimizationExemption() {
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.M) {
            val intent = Intent(Settings.ACTION_REQUEST_IGNORE_BATTERY_OPTIMIZATIONS).apply {
                data = Uri.parse("package:$packageName")
            }
            startActivity(intent)
        }
    }

    /**
     * Gets current memory pressure level.
     * Returns an integer representing the trim level:
     * 0 = TRIM_MEMORY_RUNNING_MODERATE
     * 1 = TRIM_MEMORY_RUNNING_LOW
     * 2 = TRIM_MEMORY_RUNNING_CRITICAL
     * 3 = TRIM_MEMORY_BACKGROUND_MODERATE
     * 4 = TRIM_MEMORY_BACKGROUND_LOW
     * 5 = TRIM_MEMORY_BACKGROUND_CRITICAL
     * -1 = Unknown/Unable to determine
     */
    private fun getMemoryPressureLevel(): Int {
        return try {
            val activityManager = getSystemService(Context.ACTIVITY_SERVICE) as ActivityManager
            val memoryInfo = ActivityManager.MemoryInfo()
            activityManager.getMemoryInfo(memoryInfo)
            
            // Calculate available memory percentage
            val totalMem = memoryInfo.totalMem
            val availMem = memoryInfo.availMem
            val availPercent = (availMem.toDouble() / totalMem.toDouble()) * 100
            
            when {
                availPercent < 10 -> 2 // CRITICAL
                availPercent < 20 -> 1 // LOW
                availPercent < 30 -> 0 // MODERATE
                else -> -1 // Normal
            }
        } catch (e: Exception) {
            -1
        }
    }

    /**
     * Called when the system is experiencing memory pressure.
     * We forward this to Flutter so it can pause non-essential services.
     */
    override fun onTrimMemory(level: Int) {
        super.onTrimMemory(level)
        
        // Forward memory pressure level to Flutter
        MethodChannel(flutterEngine?.dartExecutor?.binaryMessenger, MEMORY_CHANNEL)
            .invokeMethod("onMemoryPressure", level)
    }
}
