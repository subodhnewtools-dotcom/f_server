import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:permission_handler/permission_handler.dart';
import '../../providers/setup_provider.dart';

class PermissionsScreen extends ConsumerStatefulWidget {
  final VoidCallback onComplete;

  const PermissionsScreen({super.key, required this.onComplete});

  @override
  ConsumerState<PermissionsScreen> createState() => _PermissionsScreenState();
}

class _PermissionsScreenState extends ConsumerState<PermissionsScreen> {
  bool _storageGranted = false;
  bool _notificationGranted = false;
  bool _batteryOptimizationExempt = false;

  @override
  void initState() {
    super.initState();
    _checkPermissions();
  }

  Future<void> _checkPermissions() async {
    final storageStatus = await Permission.storage.status;
    final notificationStatus = await Permission.notification.status;
    
    setState(() {
      _storageGranted = storageStatus.isGranted;
      _notificationGranted = notificationStatus.isGranted;
      // Battery optimization check would need platform channel
      _batteryOptimizationExempt = true; // Placeholder
    });
  }

  Future<void> _requestStorage() async {
    final status = await Permission.storage.request();
    setState(() {
      _storageGranted = status.isGranted;
    });
    ref.read(setupProvider.notifier).setPermissionsGranted(_allGranted);
  }

  Future<void> _requestNotification() async {
    final status = await Permission.notification.request();
    setState(() {
      _notificationGranted = status.isGranted;
    });
    ref.read(setupProvider.notifier).setPermissionsGranted(_allGranted);
  }

  Future<void> _requestBatteryOptimization() async {
    // Would need platform channel to request battery optimization exemption
    setState(() {
      _batteryOptimizationExempt = true;
    });
    ref.read(setupProvider.notifier).setPermissionsGranted(_allGranted);
  }

  bool get _allGranted => 
      _storageGranted && 
      _notificationGranted && 
      _batteryOptimizationExempt;

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      body: SafeArea(
        child: Padding(
          padding: const EdgeInsets.all(24.0),
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.stretch,
            children: [
              const SizedBox(height: 32),
              Icon(
                Icons.security,
                size: 64,
                color: Theme.of(context).colorScheme.primary,
              ),
              const SizedBox(height: 24),
              const Text(
                'Permissions Required',
                style: TextStyle(
                  fontSize: 28,
                  fontWeight: FontWeight.bold,
                ),
              ),
              const SizedBox(height: 12),
              Text(
                'PocketServer needs these permissions to run the web server in the background.',
                style: Theme.of(context).textTheme.bodyLarge?.copyWith(
                      color: Colors.grey[600],
                    ),
              ),
              const SizedBox(height: 48),
              _buildPermissionTile(
                icon: Icons.folder,
                title: 'Storage Access',
                subtitle: 'Required to store project files and databases',
                isGranted: _storageGranted,
                onGrant: _requestStorage,
              ),
              const SizedBox(height: 16),
              _buildPermissionTile(
                icon: Icons.notifications,
                title: 'Notifications',
                subtitle: 'Show server status in notification bar',
                isGranted: _notificationGranted,
                onGrant: _requestNotification,
              ),
              const SizedBox(height: 16),
              _buildPermissionTile(
                icon: Icons.battery_full,
                title: 'Battery Optimization Exemption',
                subtitle: 'Keep server running when screen is off',
                isGranted: _batteryOptimizationExempt,
                onGrant: _requestBatteryOptimization,
              ),
              const Spacer(),
              if (!_allGranted)
                Container(
                  padding: const EdgeInsets.all(16),
                  decoration: BoxDecoration(
                    color: Colors.amber[50],
                    borderRadius: BorderRadius.circular(8),
                    border: Border.all(color: Colors.amber[200]!),
                  ),
                  child: Row(
                    children: [
                      Icon(Icons.info_outline, color: Colors.amber[700]),
                      const SizedBox(width: 12),
                      Expanded(
                        child: Text(
                          'All permissions are required to continue.',
                          style: TextStyle(color: Colors.amber[900]),
                        ),
                      ),
                    ],
                  ),
                ),
              const SizedBox(height: 24),
              FilledButton(
                onPressed: _allGranted ? widget.onComplete : null,
                child: const Padding(
                  padding: EdgeInsets.all(16.0),
                  child: Text('Continue'),
                ),
              ),
            ],
          ),
        ),
      ),
    );
  }

  Widget _buildPermissionTile({
    required IconData icon,
    required String title,
    required String subtitle,
    required bool isGranted,
    required VoidCallback onGrant,
  }) {
    return Card(
      elevation: isGranted ? 0 : 2,
      color: isGranted ? Colors.green[50] : null,
      child: ListTile(
        leading: Icon(
          icon,
          color: isGranted ? Colors.green : null,
        ),
        title: Row(
          children: [
            Expanded(child: Text(title)),
            if (isGranted)
              Icon(Icons.check_circle, color: Colors.green[700], size: 20),
          ],
        ),
        subtitle: Text(subtitle),
        trailing: isGranted
            ? null
            : FilledButton.tonal(
                onPressed: onGrant,
                child: const Text('Grant'),
              ),
      ),
    );
  }
}
