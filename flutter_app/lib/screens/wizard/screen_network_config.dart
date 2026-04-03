import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:freezed_annotation/freezed_annotation.dart';

part 'screen_network_config.freezed.dart';

@freezed
class NetworkConfig with _$NetworkConfig {
  const factory NetworkConfig({
    @Default(8080) int httpPort,
    @Default(8443) int httpsPort,
    @Default(3306) int mysqlPort,
    @Default(6379) int redisPort,
    @Default(9000) int haproxyStatsPort,
    @Default(true) bool bindLocalhostOnly,
    String? cloudflareTunnelToken,
    String? peerRelayUrl,
  }) = _NetworkConfig;
}

class NetworkConfigScreen extends ConsumerStatefulWidget {
  final Function(NetworkConfig) onComplete;
  final VoidCallback? onBack;

  const NetworkConfigScreen({
    super.key,
    required this.onComplete,
    this.onBack,
  });

  @override
  ConsumerState<NetworkConfigScreen> createState() =>
      _NetworkConfigScreenState();
}

class _NetworkConfigScreenState extends ConsumerState<NetworkConfigScreen> {
  late NetworkConfig _config;
  final _formKey = GlobalKey<FormState>();

  @override
  void initState() {
    super.initState();
    _config = const NetworkConfig();
  }

  bool get _isValid =>
      _config.httpPort >= 1024 &&
      _config.httpPort <= 65535 &&
      _config.httpsPort >= 1024 &&
      _config.httpsPort <= 65535 &&
      _config.mysqlPort >= 1024 &&
      _config.mysqlPort <= 65535 &&
      _config.redisPort >= 1024 &&
      _config.redisPort <= 65535 &&
      _config.haproxyStatsPort >= 1024 &&
      _config.haproxyStatsPort <= 65535;

  String? _validatePort(int? value, String name) {
    if (value == null) return '$name is required';
    if (value < 1024) return '$name must be >= 1024';
    if (value > 65535) return '$name must be <= 65535';
    return null;
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      appBar: AppBar(
        title: const Text('Network Configuration'),
        leading: widget.onBack != null
            ? IconButton(
                icon: const Icon(Icons.arrow_back),
                onPressed: widget.onBack,
              )
            : null,
      ),
      body: Form(
        key: _formKey,
        child: ListView(
          padding: const EdgeInsets.all(24.0),
          children: [
            const Text(
              'Configure Network Ports',
              style: TextStyle(
                fontSize: 24,
                fontWeight: FontWeight.bold,
              ),
            ),
            const SizedBox(height: 8),
            Text(
              'These ports will be used inside the proot environment.',
              style: Theme.of(context).textTheme.bodyMedium?.copyWith(
                    color: Colors.grey[600],
                  ),
            ),
            const SizedBox(height: 32),
            _buildPortField(
              label: 'HTTP Port',
              icon: Icons.http,
              value: _config.httpPort.toString(),
              onChanged: (value) {
                setState(() {
                  _config = _config.copyWith(httpPort: int.tryParse(value) ?? 8080);
                });
              },
              validator: (v) => _validatePort(_config.httpPort, 'HTTP port'),
            ),
            const SizedBox(height: 16),
            _buildPortField(
              label: 'HTTPS Port',
              icon: Icons.security,
              value: _config.httpsPort.toString(),
              onChanged: (value) {
                setState(() {
                  _config = _config.copyWith(httpsPort: int.tryParse(value) ?? 8443);
                });
              },
              validator: (v) => _validatePort(_config.httpsPort, 'HTTPS port'),
            ),
            const SizedBox(height: 16),
            _buildPortField(
              label: 'MySQL Port',
              icon: Icons.storage,
              value: _config.mysqlPort.toString(),
              onChanged: (value) {
                setState(() {
                  _config = _config.copyWith(mysqlPort: int.tryParse(value) ?? 3306);
                });
              },
              validator: (v) => _validatePort(_config.mysqlPort, 'MySQL port'),
            ),
            const SizedBox(height: 16),
            _buildPortField(
              label: 'Redis Port',
              icon: Icons.memory,
              value: _config.redisPort.toString(),
              onChanged: (value) {
                setState(() {
                  _config = _config.copyWith(redisPort: int.tryParse(value) ?? 6379);
                });
              },
              validator: (v) => _validatePort(_config.redisPort, 'Redis port'),
            ),
            const SizedBox(height: 16),
            _buildPortField(
              label: 'HAProxy Stats Port',
              icon: Icons.insights,
              value: _config.haproxyStatsPort.toString(),
              onChanged: (value) {
                setState(() {
                  _config = _config.copyWith(haproxyStatsPort: int.tryParse(value) ?? 9000);
                });
              },
              validator: (v) =>
                  _validatePort(_config.haproxyStatsPort, 'HAProxy stats port'),
            ),
            const SizedBox(height: 32),
            Card(
              child: SwitchListTile(
                title: const Text('Bind to localhost only'),
                subtitle: const Text('Recommended for security. Disable to allow LAN access.'),
                value: _config.bindLocalhostOnly,
                onChanged: (value) {
                  setState(() {
                    _config = _config.copyWith(bindLocalhostOnly: value);
                  });
                },
                secondary: const Icon(Icons.lock),
              ),
            ),
            const SizedBox(height: 24),
            if (!_isValid)
              Container(
                padding: const EdgeInsets.all(12),
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
                        'Please fix port validation errors above.',
                        style: TextStyle(color: Colors.red[700]),
                      ),
                    ),
                  ],
                ),
              ),
            const SizedBox(height: 16),
            FilledButton(
              onPressed: _isValid && _formKey.currentState!.validate()
                  ? () => widget.onComplete(_config)
                  : null,
              child: const Padding(
                padding: EdgeInsets.all(16.0),
                child: Text('Continue'),
              ),
            ),
          ],
        ),
      ),
    );
  }

  Widget _buildPortField({
    required String label,
    required IconData icon,
    required String value,
    required Function(String) onChanged,
    String? Function(String?)? validator,
  }) {
    return TextFormField(
      initialValue: value,
      keyboardType: TextInputType.number,
      decoration: InputDecoration(
        labelText: label,
        prefixIcon: Icon(icon),
        border: const OutlineInputBorder(),
        filled: true,
      ),
      onChanged: onChanged,
      validator: validator,
    );
  }
}
