import 'dart:convert';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:shared_preferences/shared_preferences.dart';
import '../models/setup_state.dart';
import '../services/pocketd_client.dart';

final setupProvider = StateNotifierProvider<SetupNotifier, SetupState>((ref) {
  return SetupNotifier();
});

class SetupNotifier extends StateNotifier<SetupState> {
  SetupNotifier() : super(const SetupState());

  Future<void> checkExistingSetup() async {
    try {
      final prefs = await SharedPreferences.getInstance();
      final isComplete = prefs.getBool('setup_complete') ?? false;
      
      if (isComplete) {
        final resourcesJson = prefs.getString('resources');
        final stackJson = prefs.getString('stack');
        final networkJson = prefs.getString('network');
        
        state = state.copyWith(
          isComplete: true,
          hasPermissions: true,
          resources: resourcesJson != null
              ? ResourceConfig.fromJson(jsonDecode(resourcesJson))
              : null,
          stack: stackJson != null
              ? StackSelection.fromJson(jsonDecode(stackJson))
              : null,
          network: networkJson != null
              ? NetworkConfig.fromJson(jsonDecode(networkJson))
              : null,
        );
      }
    } catch (e) {
      state = state.copyWith(error: 'Failed to load setup state: $e');
    }
  }

  void setPermissionsGranted(bool granted) {
    state = state.copyWith(hasPermissions: granted);
  }

  void setDownloading(bool downloading) {
    state = state.copyWith(isDownloading: downloading);
  }

  void updateDownloadProgress(double progress) {
    state = state.copyWith(downloadProgress: progress);
  }

  Future<void> completeSetup({
    required ResourceConfig resources,
    required StackSelection stack,
    required NetworkConfig network,
  }) async {
    try {
      state = state.copyWith(isSaving: true, error: null);

      // Save to SharedPreferences
      final prefs = await SharedPreferences.getInstance();
      
      await prefs.setString('resources', jsonEncode(resources.toJson()));
      await prefs.setString('stack', jsonEncode(stack.toJson()));
      await prefs.setString('network', jsonEncode(network.toJson()));
      await prefs.setBool('setup_complete', true);

      // Generate and save config.json via pocketd RPC
      final client = ref.read(pocketdClientProvider);
      await client.saveConfig({
        'version': '1.0',
        'resources': {
          'ram_mb': resources.ramMb,
          'storage_mb': resources.storageMb,
          'cpu_percent': resources.cpuPercent,
          'ports': {
            'http': network.httpPort,
            'https': network.httpsPort,
            'mysql': network.mysqlPort,
            'redis': network.redisPort,
            'haproxy_stats': network.haproxyStatsPort,
          },
        },
        'stack': {
          'php': stack.php,
          'nodejs': stack.nodejs,
          'redis': stack.redis,
          'python': stack.python,
        },
        'network': {
          'bind_localhost_only': network.bindLocalhostOnly,
          'cloudflare_tunnel_token': network.cloudflareTunnelToken,
          'peer_relay_url': network.peerRelayUrl,
        },
      });

      state = state.copyWith(
        isComplete: true,
        isSaving: false,
        resources: resources,
        stack: stack,
        network: network,
      );
    } catch (e) {
      state = state.copyWith(
        isSaving: false,
        error: 'Failed to save configuration: $e',
      );
    }
  }

  Future<void> resetSetup() async {
    try {
      final prefs = await SharedPreferences.getInstance();
      await prefs.clear();
      state = const SetupState();
    } catch (e) {
      state = state.copyWith(error: 'Failed to reset setup: $e');
    }
  }
}

// Extension methods for JSON serialization
extension ResourceConfigExtension on ResourceConfig {
  Map<String, dynamic> toJson() => {
        'ram_mb': ramMb,
        'storage_mb': storageMb,
        'cpu_percent': cpuPercent,
      };

  static ResourceConfig fromJson(Map<String, dynamic> json) => ResourceConfig(
        ramMb: json['ram_mb'] as int,
        storageMb: json['storage_mb'] as int,
        cpuPercent: json['cpu_percent'] as int,
      );
}

extension StackSelectionExtension on StackSelection {
  Map<String, dynamic> toJson() => {
        'php': php,
        'nodejs': nodejs,
        'redis': redis,
        'python': python,
      };

  static StackSelection fromJson(Map<String, dynamic> json) => StackSelection(
        php: json['php'] as bool,
        nodejs: json['nodejs'] as bool,
        redis: json['redis'] as bool,
        python: json['python'] as bool,
      );
}

extension NetworkConfigExtension on NetworkConfig {
  Map<String, dynamic> toJson() => {
        'http_port': httpPort,
        'https_port': httpsPort,
        'mysql_port': mysqlPort,
        'redis_port': redisPort,
        'haproxy_stats_port': haproxyStatsPort,
        'bind_localhost_only': bindLocalhostOnly,
        'cloudflare_tunnel_token': cloudflareTunnelToken,
        'peer_relay_url': peerRelayUrl,
      };

  static NetworkConfig fromJson(Map<String, dynamic> json) => NetworkConfig(
        httpPort: json['http_port'] as int,
        httpsPort: json['https_port'] as int,
        mysqlPort: json['mysql_port'] as int,
        redisPort: json['redis_port'] as int,
        haproxyStatsPort: json['haproxy_stats_port'] as int,
        bindLocalhostOnly: json['bind_localhost_only'] as bool,
        cloudflareTunnelToken: json['cloudflare_tunnel_token'] as String?,
        peerRelayUrl: json['peer_relay_url'] as String?,
      );
}
