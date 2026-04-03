import 'package:flutter/foundation.dart';
import 'package:freezed_annotation/freezed_annotation.dart';

part 'setup_state.freezed.dart';

@freezed
class ResourceConfig with _$ResourceConfig {
  const factory ResourceConfig({
    @Default(512) int ramMb,
    @Default(5120) int storageMb,
    @Default(30) int cpuPercent,
  }) = _ResourceConfig;
}

@freezed
class StackSelection with _$StackSelection {
  const factory StackSelection({
    @Default(true) bool php,
    @Default(false) bool nodejs,
    @Default(false) bool redis,
    @Default(false) bool python,
  }) = _StackSelection;
}

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

@freezed
class SetupState with _$SetupState {
  const factory SetupState({
    @Default(false) bool isComplete,
    @Default(false) bool hasPermissions,
    @Default(false) bool isDownloading,
    @Default(0.0) double downloadProgress,
    @Default(false) bool isSaving,
    String? error,
    ResourceConfig? resources,
    StackSelection? stack,
    NetworkConfig? network,
  }) = _SetupState;
}
