import 'dart:async';
import 'dart:convert';
import 'dart:io';

import 'package:crypto/crypto.dart';
import 'package:uuid/uuid.dart';

/// JSON-RPC 2.0 client for communicating with pocketd over Unix domain socket.
/// 
/// This client handles connection management, request/response correlation,
/// and automatic reconnection if pocketd restarts.
class PocketdClient {
  static const String _socketPath = '/tmp/pocket.sock';
  
  Socket? _socket;
  bool _isConnected = false;
  final Map<String, Completer<Map<String, dynamic>>> _pendingRequests = {};
  final StreamController<Map<String, dynamic>> _responseStream = 
      StreamController<Map<String, dynamic>>.broadcast();
  final StreamController<String> _logStream = 
      StreamController<String>.broadcast();
  
  bool get isConnected => _isConnected;
  Stream<Map<String, dynamic>> get responses => _responseStream.stream;
  Stream<String> get logs => _logStream.stream;

  /// Connects to the pocketd Unix domain socket.
  /// 
  /// Returns true if connection succeeds, false otherwise.
  Future<bool> connect({Duration timeout = const Duration(seconds: 5)}) async {
    try {
      await disconnect();
      
      final socket = await Socket.connect(
        InternetAddress(_socketPath, type: InternetAddressType.unix),
        0,
        timeout: timeout,
      );
      
      _socket = socket;
      _isConnected = true;
      
      // Start listening for responses
      _listenToSocket();
      
      return true;
    } catch (e) {
      _isConnected = false;
      _socket = null;
      return false;
    }
  }

  /// Disconnects from the pocketd socket.
  Future<void> disconnect() async {
    _isConnected = false;
    
    // Complete all pending requests with error
    for (final entry in _pendingRequests.entries) {
      entry.value.completeError(
        Exception('Connection closed'),
      );
    }
    _pendingRequests.clear();
    
    await _socket?.close();
    _socket = null;
  }

  void _listenToSocket() {
    if (_socket == null) return;
    
    final buffer = StringBuffer();
    
    _socket!.listen(
      (List<int> data) {
        buffer.write(String.fromCharCodes(data));
        
        // Process complete lines (JSON-RPC responses are newline-delimited)
        final lines = buffer.toString().split('\n');
        buffer.clear();
        
        for (int i = 0; i < lines.length - 1; i++) {
          final line = lines[i].trim();
          if (line.isEmpty) continue;
          
          try {
            final response = jsonDecode(line) as Map<String, dynamic>;
            _handleResponse(response);
          } catch (e) {
            _logStream.add('Failed to parse response: $e');
          }
        }
        
        // Keep the last incomplete line in buffer
        if (lines.isNotEmpty) {
          buffer.write(lines.last);
        }
      },
      onError: (error) {
        _isConnected = false;
        _logStream.add('Socket error: $error');
        _responseStream.addError(error);
      },
      onDone: () {
        _isConnected = false;
        _logStream.add('Socket closed');
        _responseStream.add({'event': 'disconnected'});
      },
    );
  }

  void _handleResponse(Map<String, dynamic> response) {
    final id = response['id'] as String?;
    
    if (id != null && _pendingRequests.containsKey(id)) {
      final completer = _pendingRequests.remove(id)!;
      
      if (response.containsKey('error')) {
        completer.completeError(
          JsonRpcException(
            code: (response['error'] as Map)['code'] as int,
            message: (response['error'] as Map)['message'] as String,
            data: (response['error'] as Map)['data'],
          ),
        );
      } else {
        completer.complete(response['result'] as Map<String, dynamic>? ?? {});
      }
    } else {
      // Unsolicited response (e.g., from streaming)
      _responseStream.add(response);
    }
  }

  /// Sends a JSON-RPC 2.0 request and returns the result.
  /// 
  /// [method] is the RPC method name (e.g., 'daemon.status').
  /// [params] are the method parameters.
  Future<Map<String, dynamic>> call(
    String method, {
    Map<String, dynamic> params = const {},
  }) async {
    if (!_isConnected || _socket == null) {
      throw Exception('Not connected to pocketd');
    }
    
    final requestId = const Uuid().v4();
    final completer = Completer<Map<String, dynamic>>();
    _pendingRequests[requestId] = completer;
    
    final request = {
      'jsonrpc': '2.0',
      'id': requestId,
      'method': method,
      'params': params,
    };
    
    final requestJson = jsonEncode(request);
    _socket!.write('$requestJson\n');
    await _socket!.flush();
    
    return completer.future.timeout(
      const Duration(seconds: 30),
      onTimeout: () {
        _pendingRequests.remove(requestId);
        throw TimeoutException('Request timed out: $method');
      },
    );
  }

  /// Daemon control methods
  
  Future<Map<String, dynamic>> daemonStatus() async {
    return call('daemon.status');
  }

  Future<Map<String, dynamic>> daemonStart() async {
    return call('daemon.start');
  }

  Future<Map<String, dynamic>> daemonStop({bool graceful = true}) async {
    return call('daemon.stop', params: {'graceful': graceful});
  }

  Future<Map<String, dynamic>> daemonRestart() async {
    return call('daemon.restart');
  }

  /// Metrics methods
  
  Future<Map<String, dynamic>> metricsSnapshot() async {
    return call('metrics.snapshot');
  }

  Future<List<Map<String, dynamic>>> metricsServices() async {
    final result = await call('metrics.services');
    return (result['services'] as List?)
            ?.map((e) => e as Map<String, dynamic>)
            .toList() ??
        [];
  }

  /// Project management methods
  
  Future<List<Map<String, dynamic>>> projectList() async {
    final result = await call('project.list');
    return (result['projects'] as List?)
            ?.map((e) => e as Map<String, dynamic>)
            .toList() ??
        [];
  }

  Future<Map<String, dynamic>> projectCreate({
    required String name,
    required List<String> stack,
    String? domain,
  }) async {
    return call('project.create', params: {
      'name': name,
      'stack': stack,
      if (domain != null) 'domain': domain,
    });
  }

  Future<Map<String, dynamic>> projectDelete({required String slug}) async {
    return call('project.delete', params: {'slug': slug});
  }

  Future<Map<String, dynamic>> projectStart({required String slug}) async {
    return call('project.start', params: {'slug': slug});
  }

  Future<Map<String, dynamic>> projectStop({required String slug}) async {
    return call('project.stop', params: {'slug': slug});
  }

  /// API key management methods
  
  Future<Map<String, dynamic>> apikeyCreate({
    required String name,
    required List<String> scopes,
    DateTime? expiresAt,
  }) async {
    return call('apikey.create', params: {
      'name': name,
      'scopes': scopes,
      if (expiresAt != null) 'expires_at': expiresAt.toIso8601String(),
    });
  }

  Future<List<Map<String, dynamic>>> apikeyList() async {
    final result = await call('apikey.list');
    return (result['keys'] as List?)
            ?.map((e) => e as Map<String, dynamic>)
            .toList() ??
        [];
  }

  Future<Map<String, dynamic>> apikeyRevoke({required String keyId}) async {
    return call('apikey.revoke', params: {'key_id': keyId});
  }

  /// Backup methods
  
  Future<Map<String, dynamic>> backupCreate({
    required String slug,
    String type = 'full',
    String? destinationId,
  }) async {
    return call('backup.create', params: {
      'slug': slug,
      'type': type,
      if (destinationId != null) 'destination_id': destinationId,
    });
  }

  Future<List<Map<String, dynamic>>> backupList({required String slug}) async {
    final result = await call('backup.list', params: {'slug': slug});
    return (result['backups'] as List?)
            ?.map((e) => e as Map<String, dynamic>)
            .toList() ??
        [];
  }

  Future<Map<String, dynamic>> backupRestore({
    required String backupId,
    required String slug,
  }) async {
    return call('backup.restore', params: {
      'backup_id': backupId,
      'slug': slug,
    });
  }

  Future<Map<String, dynamic>> backupJobStatus({required String jobId}) async {
    return call('backup.job_status', params: {'job_id': jobId});
  }

  /// Peer management methods
  
  Future<List<Map<String, dynamic>>> peerList() async {
    final result = await call('peer.list');
    return (result['peers'] as List?)
            ?.map((e) => e as Map<String, dynamic>)
            .toList() ??
        [];
  }

  Future<Map<String, dynamic>> peerAdd({
    required String certPem,
    required String name,
  }) async {
    return call('peer.add', params: {
      'cert_pem': certPem,
      'name': name,
    });
  }

  Future<Map<String, dynamic>> peerRemove({required String peerId}) async {
    return call('peer.remove', params: {'peer_id': peerId});
  }

  Future<List<Map<String, dynamic>>> peerScanLan() async {
    final result = await call('peer.scan_lan');
    return (result['peers'] as List?)
            ?.map((e) => e as Map<String, dynamic>)
            .toList() ??
        [];
  }

  /// Load balancer methods
  
  Future<Map<String, dynamic>> lbSetMode({
    required String mode,
  }) async {
    assert(['round_robin', 'least_conn', 'primary_standby'].contains(mode));
    return call('lb.set_mode', params: {'mode': mode});
  }

  Future<Map<String, dynamic>> lbStatus() async {
    return call('lb.status');
  }

  /// Log methods
  
  Future<List<Map<String, dynamic>>> logsTail({
    String? service,
    int lines = 100,
  }) async {
    final result = await call('logs.tail', params: {
      if (service != null) 'service': service,
      'lines': lines,
    });
    return (result['logs'] as List?)
            ?.map((e) => e as Map<String, dynamic>)
            .toList() ??
        [];
  }

  Stream<Map<String, dynamic>> logsStream({String? service}) async* {
    // Subscribe to the log stream
    final subscription = _logStream.stream.listen((log) {
      // Parse and emit as JSON
      try {
        yield jsonDecode(log) as Map<String, dynamic>;
      } catch (_) {
        // Emit as raw text
        yield {'message': log};
      }
    });
    
    yield* Stream<Map<String, dynamic>>.eventTransformed(
      _responseStream.stream,
      (stream) => stream.transform(
        StreamTransformer.fromHandlers(
          handleData: (data, sink) {
            if (data.containsKey('log')) {
              sink.add(data);
            }
          },
        ),
      ),
    );
    
    await subscription.cancel();
  }

  /// Attempts to reconnect to pocketd with exponential backoff.
  Future<void> reconnect({
    int maxRetries = 5,
    Duration baseDelay = const Duration(seconds: 1),
  }) async {
    int retryCount = 0;
    
    while (retryCount < maxRetries) {
      final connected = await connect();
      if (connected) {
        _logStream.add('Reconnected successfully');
        return;
      }
      
      retryCount++;
      if (retryCount < maxRetries) {
        final delay = baseDelay * (1 << (retryCount - 1));
        _logStream.add('Reconnect attempt $retryCount failed, retrying in ${delay.inSeconds}s');
        await Future.delayed(delay);
      }
    }
    
    throw Exception('Failed to reconnect after $maxRetries attempts');
  }
}

/// Exception thrown when a JSON-RPC error is received.
class JsonRpcException implements Exception {
  final int code;
  final String message;
  final dynamic data;

  JsonRpcException({
    required this.code,
    required this.message,
    this.data,
  });

  @override
  String toString() => 'JsonRpcException($code): $message';
}
