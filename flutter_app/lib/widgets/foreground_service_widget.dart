import 'package:flutter/material.dart';
import 'package:flutter_foreground_task/flutter_foreground_task.dart';
import '../services/foreground_service.dart';

/// Provider mixin that manages the foreground service lifecycle.
/// Use this mixin in widgets that need to start/stop the daemon service.
mixin ForegroundServiceMixin<T extends StatefulWidget> on State<T> {
  bool _isServiceRunning = false;

  bool get isServiceRunning => _isServiceRunning;

  @override
  void initState() {
    super.initState();
    _checkServiceStatus();
    
    // Listen for service state changes
    FlutterForegroundTask.addEventCallback(_onForegroundEvent);
  }

  @override
  void dispose() {
    FlutterForegroundTask.removeEventCallback(_onForegroundEvent);
    super.dispose();
  }

  Future<void> _checkServiceStatus() async {
    final isRunning = await FlutterForegroundTask.isRunningService;
    setState(() {
      _isServiceRunning = isRunning;
    });
  }

  void _onForegroundEvent(ForegroundEvent event) {
    switch (event) {
      case ForegroundEvent.onStart:
        setState(() {
          _isServiceRunning = true;
        });
        break;
      case ForegroundEvent.onStop:
        setState(() {
          _isServiceRunning = false;
        });
        break;
      default:
        break;
    }
  }

  /// Starts the foreground service to keep pocketd alive.
  /// This should be called when the user enables the server.
  Future<void> startForegroundService() async {
    if (_isServiceRunning) return;

    // Request notification permission first (Android 13+)
    if (!await FlutterForegroundTask.isNotificationPermissionGranted) {
      final granted = await FlutterForegroundTask.requestNotificationPermission();
      if (!granted) {
        if (mounted) {
          ScaffoldMessenger.of(context).showSnackBar(
            const SnackBar(
              content: Text('Notification permission required for background operation'),
              backgroundColor: Colors.red,
            ),
          );
        }
        return;
      }
    }

    // Start the service
    await FlutterForegroundTask.startService(
      notificationTitle: 'PocketServer Daemon',
      notificationText: 'Server is running',
      iconData: const IconData(Icons.dns, fontPackage: 'material'),
      callback: callback,
    );

    setState(() {
      _isServiceRunning = true;
    });

    if (mounted) {
      ScaffoldMessenger.of(context).showSnackBar(
        const SnackBar(
          content: Text('Background service started'),
          backgroundColor: Colors.green,
        ),
      );
    }
  }

  /// Stops the foreground service.
  /// This should be called when the user disables the server.
  Future<void> stopForegroundService() async {
    if (!_isServiceRunning) return;

    await FlutterForegroundTask.stopService();

    setState(() {
      _isServiceRunning = false;
    });

    if (mounted) {
      ScaffoldMessenger.of(context).showSnackBar(
        const SnackBar(
          content: Text('Background service stopped'),
        ),
      );
    }
  }

  /// Toggles the foreground service on/off.
  Future<void> toggleForegroundService() async {
    if (_isServiceRunning) {
      await stopForegroundService();
    } else {
      await startForegroundService();
    }
  }
}

/// Widget that displays the current foreground service status.
class ServiceStatusIndicator extends StatelessWidget {
  final bool isRunning;

  const ServiceStatusIndicator({
    Key? key,
    required this.isRunning,
  }) : super(key: key);

  @override
  Widget build(BuildContext context) {
    return Container(
      padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 6),
      decoration: BoxDecoration(
        color: isRunning ? Colors.green.shade100 : Colors.grey.shade200,
        borderRadius: BorderRadius.circular(16),
      ),
      child: Row(
        mainAxisSize: MainAxisSize.min,
        children: [
          Container(
            width: 8,
            height: 8,
            decoration: BoxDecoration(
              shape: BoxShape.circle,
              color: isRunning ? Colors.green : Colors.grey,
            ),
          ),
          const SizedBox(width: 6),
          Text(
            isRunning ? 'Service Active' : 'Service Inactive',
            style: TextStyle(
              fontSize: 12,
              fontWeight: FontWeight.w600,
              color: isRunning ? Colors.green.shade800 : Colors.grey.shade700,
            ),
          ),
        ],
      ),
    );
  }
}
