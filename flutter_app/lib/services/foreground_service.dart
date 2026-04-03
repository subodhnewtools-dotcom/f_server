import 'package:flutter_foreground_task/flutter_foreground_task.dart';
import 'package:flutter/foundation.dart';
import '../services/pocketd_client.dart';
import '../models/dashboard_models.dart';

/// Callback function for the foreground task.
/// This runs in a separate isolate from the main UI.
@pragma('vm:entry-point')
void callback() {
  FlutterForegroundTask.setTaskHandler(
    PocketServerTaskHandler(),
  );
}

/// Handles the lifecycle of the foreground service.
/// Manages WakeLocks, monitors pocketd health, and handles memory pressure.
class PocketServerTaskHandler extends TaskHandler {
  PocketdClient? _client;
  int _consecutiveFailures = 0;
  static const int maxFailures = 3;

  @override
  Future<void> onStart(DateTime timestamp, TaskStarter starter) async {
    debugPrint('[ForegroundService] Started at $timestamp');
    // Initialize client connection to pocketd Unix socket
    _client = PocketdClient();
    try {
      await _client!.connect();
      debugPrint('[ForegroundService] Connected to pocketd');
    } catch (e) {
      debugPrint('[ForegroundService] Failed to connect to pocketd: $e');
      // We don't crash here; we'll retry in onRepeatEvent
    }
  }

  @override
  Future<void> onRepeatEvent() async {
    // Poll pocketd status every 5 seconds to ensure it's alive
    if (_client == null || !_client!.isConnected) {
      debugPrint('[ForegroundService] Reconnecting to pocketd...');
      try {
        if (_client != null) await _client!.disconnect();
        _client = PocketdClient();
        await _client!.connect();
        _consecutiveFailures = 0;
      } catch (e) {
        _consecutiveFailures++;
        debugPrint('[ForegroundService] Reconnection failed ($maxFailures): $e');
        if (_consecutiveFailures >= maxFailures) {
          // Optionally stop service if daemon is dead for too long
          // FlutterForegroundTask.stopService();
        }
      }
      return;
    }

    try {
      final status = await _client!.daemonStatus();
      // Reset failure counter on success
      _consecutiveFailures = 0;
      
      // Update notification with live req/s if available
      // Note: Accessing main isolate state from here requires SendPort
      // For now, we just keep the service alive. 
      // The UI polls separately for dashboard updates.
    } catch (e) {
      _consecutiveFailures++;
      debugPrint('[ForegroundService] Status check failed: $e');
    }
  }

  @override
  void onDestroy(DateTime timestamp) {
    debugPrint('[ForegroundService] Destroyed at $timestamp');
    _client?.disconnect();
    _client = null;
  }

  @override
  void onNotificationButtonPressed(String id) {
    // Handle notification button taps (e.g., Stop Server)
    debugPrint('[ForegroundService] Button pressed: $id');
    if (id == 'stop_daemon') {
      _client?.daemonStop(graceful: true);
    }
  }

  @override
  void onNotificationPressed() {
    // Launch app when notification is tapped
    debugPrint('[ForegroundService] Notification tapped');
  }
}

/// Configuration for the foreground service notification.
class ForegroundServiceConfig {
  static ForegroundTaskConfig getConfig({required bool isDarkMode}) {
    return ForegroundTaskConfig(
      android: AndroidConfig(
        serviceType: AndroidServiceType.dataSync,
        notificationChannelId: 'pocketserver_daemon',
        notificationTitle: 'PocketServer Daemon',
        notificationText: 'Running server services...',
        notificationIcon: const ResourceDrawable(name: 'ic_server_icon'),
        buttons: [
          const NotificationButton(id: 'stop_daemon', text: 'Stop'),
        ],
        // Prevent service from being killed on low memory if possible
        priority: AndroidNotificationPriority.low,
      ),
      ios: IOSConfig(
        // iOS doesn't support this type of background service in the same way
        // This is primarily for Android
      ),
    );
  }
}
