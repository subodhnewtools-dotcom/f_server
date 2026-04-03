import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import '../../models/setup_state.dart';
import '../../providers/setup_provider.dart';

class ResourcesScreen extends ConsumerStatefulWidget {
  final Function(ResourceConfig) onComplete;
  final VoidCallback? onBack;

  const ResourcesScreen({
    super.key,
    required this.onComplete,
    this.onBack,
  });

  @override
  ConsumerState<ResourcesScreen> createState() => _ResourcesScreenState();
}

class _ResourcesScreenState extends ConsumerState<ResourcesScreen> {
  late ResourceConfig _config;
  int _totalRamMb = 4096;
  int _totalCpuCores = 8;
  int _totalStorageMb = 65536;

  @override
  void initState() {
    super.initState();
    _config = const ResourceConfig();
    _detectSystemResources();
  }

  Future<void> _detectSystemResources() async {
    // In production, these would come from pocketd RPC reading /proc/meminfo
    // For now, use reasonable defaults
    setState(() {
      _totalRamMb = 4096;
      _totalCpuCores = 8;
      _totalStorageMb = 65536;
      
      // Set defaults based on detected resources
      _config = ResourceConfig(
        ramMb: (_totalRamMb * 0.125).toInt().clamp(128, 2048),
        storageMb: (_totalStorageMb * 0.08).toInt().clamp(512, 10240),
        cpuPercent: 30,
      );
    });
  }

  bool get _isValid =>
      _config.ramMb >= 128 &&
      _config.ramMb <= 2048 &&
      _config.storageMb >= 512 &&
      _config.storageMb <= 10240 &&
      _config.cpuPercent >= 10 &&
      _config.cpuPercent <= 80;

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      appBar: AppBar(
        title: const Text('Resource Allocation'),
        leading: widget.onBack != null
            ? IconButton(
                icon: const Icon(Icons.arrow_back),
                onPressed: widget.onBack,
              )
            : null,
      ),
      body: ListView(
        padding: const EdgeInsets.all(24.0),
        children: [
          const Text(
            'Allocate Resources',
            style: TextStyle(
              fontSize: 24,
              fontWeight: FontWeight.bold,
            ),
          ),
          const SizedBox(height: 8),
          Text(
            'How much of your device resources should PocketServer use?',
            style: Theme.of(context).textTheme.bodyMedium?.copyWith(
                  color: Colors.grey[600],
                ),
          ),
          const SizedBox(height: 32),
          _buildRamSlider(),
          const SizedBox(height: 24),
          _buildStorageSlider(),
          const SizedBox(height: 24),
          _buildCpuSlider(),
          const SizedBox(height: 32),
          if (!_isValid)
            Container(
              padding: const EdgeInsets.all(16),
              decoration: BoxDecoration(
                color: Colors.red[50],
                borderRadius: BorderRadius.circular(8),
              ),
              child: Row(
                children: [
                  Icon(Icons.error, color: Colors.red[700]),
                  const SizedBox(width: 12),
                  Expanded(
                    child: Text(
                      'Please adjust values to valid ranges.',
                      style: TextStyle(color: Colors.red[700]),
                    ),
                  ),
                ],
              ),
            ),
          const SizedBox(height: 24),
          FilledButton(
            onPressed: _isValid ? () => widget.onComplete(_config) : null,
            child: const Padding(
              padding: EdgeInsets.all(16.0),
              child: Text('Continue'),
            ),
          ),
        ],
      ),
    );
  }

  Widget _buildRamSlider() {
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Row(
          mainAxisAlignment: MainAxisAlignment.spaceBetween,
          children: [
            Row(
              children: [
                Icon(Icons.memory, size: 20),
                const SizedBox(width: 8),
                const Text('RAM', style: TextStyle(fontWeight: FontWeight.w600)),
              ],
            ),
            Text(
              '${_config.ramMb} MB / $_totalRamMb MB',
              style: Theme.of(context).textTheme.titleMedium,
            ),
          ],
        ),
        const SizedBox(height: 12),
        Slider(
          value: _config.ramMb.toDouble(),
          min: 128,
          max: 2048,
          divisions: 38,
          label: '${_config.ramMb} MB',
          onChanged: (value) {
            setState(() {
              _config = _config.copyWith(ramMb: value.toInt());
            });
          },
        ),
        Text(
          'Recommended: ${(_totalRamMb * 0.125).toInt()} MB (${(12.5).toStringAsFixed(0)}% of total)',
          style: Theme.of(context).textTheme.bodySmall?.copyWith(
                color: Colors.grey[600],
              ),
        ),
      ],
    );
  }

  Widget _buildStorageSlider() {
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Row(
          mainAxisAlignment: MainAxisAlignment.spaceBetween,
          children: [
            Row(
              children: [
                Icon(Icons.storage, size: 20),
                const SizedBox(width: 8),
                const Text('Storage', style: TextStyle(fontWeight: FontWeight.w600)),
              ],
            ),
            Text(
              '${_config.storageMb} MB',
              style: Theme.of(context).textTheme.titleMedium,
            ),
          ],
        ),
        const SizedBox(height: 12),
        Slider(
          value: _config.storageMb.toDouble(),
          min: 512,
          max: 10240,
          divisions: 39,
          label: '${_config.storageMb} MB',
          onChanged: (value) {
            setState(() {
              _config = _config.copyWith(storageMb: value.toInt());
            });
          },
        ),
        Text(
          'For projects, databases, and backups',
          style: Theme.of(context).textTheme.bodySmall?.copyWith(
                color: Colors.grey[600],
              ),
        ),
      ],
    );
  }

  Widget _buildCpuSlider() {
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Row(
          mainAxisAlignment: MainAxisAlignment.spaceBetween,
          children: [
            Row(
              children: [
                Icon(Icons.dns, size: 20),
                const SizedBox(width: 8),
                const Text('CPU Limit', style: TextStyle(fontWeight: FontWeight.w600)),
              ],
            ),
            Text(
              '${_config.cpuPercent}%',
              style: Theme.of(context).textTheme.titleMedium,
            ),
          ],
        ),
        const SizedBox(height: 12),
        Slider(
          value: _config.cpuPercent.toDouble(),
          min: 10,
          max: 80,
          divisions: 14,
          label: '${_config.cpuPercent}%',
          onChanged: (value) {
            setState(() {
              _config = _config.copyWith(cpuPercent: value.toInt());
            });
          },
        ),
        Text(
          'Max CPU usage across $_totalCpuCores cores',
          style: Theme.of(context).textTheme.bodySmall?.copyWith(
                color: Colors.grey[600],
              ),
        ),
      ],
    );
  }
}
