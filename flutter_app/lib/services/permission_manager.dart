import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:permission_handler/permission_handler.dart';

/// Helper class to manage Android-specific permissions required by PocketServer.
class PermissionManager {
  /// Requests all required permissions for PocketServer to function.
  /// Returns a map of permission to granted status.
  static Future<Map<Permission, bool>> requestAllPermissions() async {
    final results = <Permission, bool>{};

    // Storage permissions (for accessing project files and backups)
    if (await Permission.storage.status.isDenied) {
      final storageStatus = await Permission.storage.request();
      results[Permission.storage] = storageStatus.isGranted;
    } else {
      results[Permission.storage] = await Permission.storage.isGranted;
    }

    // Manage external storage for Android 11+
    if (await Permission.manageExternalStorage.status.isDenied) {
      final manageStatus = await Permission.manageExternalStorage.request();
      results[Permission.manageExternalStorage] = manageStatus.isGranted;
    } else {
      results[Permission.manageExternalStorage] = 
          await Permission.manageExternalStorage.isGranted;
    }

    // Battery optimization exemption (critical for background operation)
    // This requires opening system settings, handled separately
    results[Permission.ignoreBatteryOptimizations] = 
        await _isBatteryOptimizationDisabled();

    // Foreground service permission (Android 13+)
    if (await Permission.notification.status.isDenied) {
      final notifStatus = await Permission.notification.request();
      results[Permission.notification] = notifStatus.isGranted;
    } else {
      results[Permission.notification] = await Permission.notification.isGranted;
    }

    return results;
  }

  /// Checks if battery optimization is disabled for this app.
  /// Returns true if the app can run in background without restrictions.
  static Future<bool> _isBatteryOptimizationDisabled() async {
    try {
      const platform = MethodChannel('com.pocketserver.app/battery');
      final result = await platform.invokeMethod('isBatteryOptimizationDisabled');
      return result as bool;
    } on PlatformException catch (e) {
      debugPrint('Failed to check battery optimization: ${e.message}');
      return false;
    }
  }

  /// Requests battery optimization exemption by opening system settings.
  /// The user must manually disable optimization for this app.
  static Future<void> requestBatteryOptimizationExemption() async {
    try {
      const platform = MethodChannel('com.pocketserver.app/battery');
      await platform.invokeMethod('requestBatteryOptimizationExemption');
    } on PlatformException catch (e) {
      debugPrint('Failed to request battery exemption: ${e.message}');
      // Fallback: open Android settings manually
      await openAppSettings();
    }
  }

  /// Checks the current status of all required permissions.
  static Future<PermissionStatusSummary> getPermissionSummary() async {
    final storage = await Permission.storage.status;
    final manageExt = await Permission.manageExternalStorage.status;
    final battery = await _isBatteryOptimizationDisabled();
    final notification = await Permission.notification.status;

    return PermissionStatusSummary(
      storageGranted: storage.isGranted,
      manageExternalStorageGranted: manageExt.isGranted,
      batteryOptimizationDisabled: battery,
      notificationGranted: notification.isGranted,
      allGranted: storage.isGranted && 
                  manageExt.isGranted && 
                  battery && 
                  notification.isGranted,
    );
  }

  /// Handles Android memory pressure signals.
  /// Called from MainActivity when onTrimMemory is received.
  static void handleMemoryPressure(int level) {
    // TRIM_MEMORY_RUNNING_CRITICAL = 6
    // When critical, we should signal pocketd to pause non-essential services
    if (level == 6) {
      debugPrint('[MemoryPressure] CRITICAL - pausing non-essential services');
      // This would communicate with pocketd to stop Redis/Node.js temporarily
      // Implementation in next phase
    }
  }
}

/// Summary of all permission statuses.
class PermissionStatusSummary {
  final bool storageGranted;
  final bool manageExternalStorageGranted;
  final bool batteryOptimizationDisabled;
  final bool notificationGranted;
  final bool allGranted;

  PermissionStatusSummary({
    required this.storageGranted,
    required this.manageExternalStorageGranted,
    required this.batteryOptimizationDisabled,
    required this.notificationGranted,
    required this.allGranted,
  });

  /// Returns a list of missing permissions for display to the user.
  List<String> getMissingPermissions() {
    final missing = <String>[];
    
    if (!storageGranted) {
      missing.add('Storage access');
    }
    if (!manageExternalStorageGranted) {
      missing.add('File management (Android 11+)');
    }
    if (!batteryOptimizationDisabled) {
      missing.add('Battery optimization exemption');
    }
    if (!notificationGranted) {
      missing.add('Notification permission');
    }
    
    return missing;
  }
}
