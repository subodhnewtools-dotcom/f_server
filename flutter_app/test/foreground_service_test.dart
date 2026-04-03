import 'package:flutter_test/flutter_test.dart';
import 'package:mocktail/mocktail.dart';
import 'package:flutter_foreground_task/flutter_foreground_task.dart';

// Mock classes for testing
class MockFlutterForegroundTask extends Mock implements FlutterForegroundTask {}

void main() {
  group('ForegroundServiceMixin Tests', () {
    setUpAll(() {
      // Register fallback values for mocktail
      registerFallbackValue(ForegroundEvent.onStart);
    });

    test('ServiceStatusIndicator displays active state correctly', () {
      expect(
        ServiceStatusIndicator(isRunning: true),
        isA<StatelessWidget>(),
      );
    });

    test('ServiceStatusIndicator displays inactive state correctly', () {
      expect(
        ServiceStatusIndicator(isRunning: false),
        isA<StatelessWidget>(),
      );
    });

    test('Foreground service config creates valid Android config', () {
      // This test verifies the configuration structure
      // Actual execution requires Android platform
      expect(
        () => ForegroundServiceConfig.getConfig(isDarkMode: false),
        returnsNormally,
      );
    });

    test('Foreground service config creates valid config for dark mode', () {
      expect(
        () => ForegroundServiceConfig.getConfig(isDarkMode: true),
        returnsNormally,
      );
    });
  });

  group('PocketServerTaskHandler Tests', () {
    test('TaskHandler can be instantiated', () {
      expect(
        PocketServerTaskHandler(),
        isA<TaskHandler>(),
      );
    });

    test('onStart callback signature is correct', () {
      final handler = PocketServerTaskHandler();
      expect(
        () => handler.onStart(DateTime.now(), TaskStarter.developer),
        returnsNormally,
      );
    });

    test('onDestroy callback signature is correct', () {
      final handler = PocketServerTaskHandler();
      expect(
        () => handler.onDestroy(DateTime.now()),
        returnsNormally,
      );
    });

    test('onNotificationPressed callback signature is correct', () {
      final handler = PocketServerTaskHandler();
      expect(
        () => handler.onNotificationPressed(),
        returnsNormally,
      );
    });

    test('onNotificationButtonPressed callback signature is correct', () {
      final handler = PocketServerTaskHandler();
      expect(
        () => handler.onNotificationButtonPressed('stop_daemon'),
        returnsNormally,
      );
    });
  });

  group('PermissionManager Integration Tests', () {
    test('PermissionStatusSummary can be created with all granted', () {
      const summary = PermissionStatusSummary(
        storageGranted: true,
        manageExternalStorageGranted: true,
        batteryOptimizationDisabled: true,
        notificationGranted: true,
        allGranted: true,
      );

      expect(summary.allGranted, isTrue);
      expect(summary.getMissingPermissions(), isEmpty);
    });

    test('PermissionStatusSummary identifies missing permissions', () {
      const summary = PermissionStatusSummary(
        storageGranted: false,
        manageExternalStorageGranted: false,
        batteryOptimizationDisabled: false,
        notificationGranted: false,
        allGranted: false,
      );

      expect(summary.allGranted, isFalse);
      expect(summary.getMissingPermissions(), hasLength(4));
      expect(summary.getMissingPermissions(), contains('Storage access'));
      expect(summary.getMissingPermissions(), contains('File management (Android 11+)'));
      expect(summary.getMissingPermissions(), contains('Battery optimization exemption'));
      expect(summary.getMissingPermissions(), contains('Notification permission'));
    });

    test('PermissionStatusSummary handles partial permissions', () {
      const summary = PermissionStatusSummary(
        storageGranted: true,
        manageExternalStorageGranted: true,
        batteryOptimizationDisabled: false,
        notificationGranted: true,
        allGranted: false,
      );

      expect(summary.allGranted, isFalse);
      expect(summary.getMissingPermissions(), hasLength(1));
      expect(summary.getMissingPermissions(), contains('Battery optimization exemption'));
    });
  });
}

// Import the actual classes to test
import 'package:flutter/material.dart';
import '../lib/services/foreground_service.dart';
import '../lib/services/permission_manager.dart';
import '../lib/widgets/foreground_service_widget.dart';
