# PocketServer Security Fixes Implementation Plan

## Executive Summary

This document outlines the comprehensive security fixes to be implemented across the PocketServer codebase, focusing on three critical areas:
1. **Error Handling** - Proper error wrapping, context preservation, and structured error responses
2. **Exception Handling** - Panic recovery, graceful degradation, and fault isolation
3. **Sensitive Data Handling** - Encryption at rest, secure memory handling, and credential protection

## Current State Analysis

### Go Codebase Issues Found:

1. **config.go**: 
   - ❌ Security fields (api_key_salt, jwt_secret) stored in plaintext
   - ⚠️ Error messages lack context wrapping
   - ✅ Good validation logic present

2. **rpc/server.go**:
   - ❌ Panic recovery doesn't log structured information
   - ❌ Error responses may leak internal details
   - ⚠️ No request size limits enforced
   - ✅ Context propagation implemented

3. **pocketd_client.dart**:
   - ❌ Generic Exception types instead of sealed hierarchy
   - ❌ Socket errors logged without sanitization
   - ⚠️ No retry limit enforcement in reconnect logic

### Flutter Codebase Issues Found:

1. **No Result/Either pattern** - Uses null and exceptions interchangeably
2. **Missing error boundaries** - UI can crash on unexpected data
3. **Credential storage** - Relies on platform channels not yet implemented

## Phase 1: Immediate Critical Fixes

### 1.1 Encrypt Sensitive Config Fields (Go)

**File**: `pocketd/internal/config/config.go`

**Changes Required**:
- Add encryption/decryption methods using AES-256-GCM
- Derive encryption key from Android Keystore (via platform channel)
- Encrypt `Security` struct before saving to disk
- Decrypt after loading from disk
- Store only ciphertext in JSON file

**Implementation**:
```go
package config

import (
    "crypto/aes"
    "crypto/cipher"
    "crypto/rand"
    "encoding/base64"
    "errors"
    "io"
)

// encryptSecret encrypts a string using AES-256-GCM
func encryptSecret(plaintext string, key []byte) (string, error) {
    block, err := aes.NewCipher(key)
    if err != nil {
        return "", fmt.Errorf("failed to create cipher: %w", err)
    }
    
    gcm, err := cipher.NewGCM(block)
    if err != nil {
        return "", fmt.Errorf("failed to create GCM: %w", err)
    }
    
    nonce := make([]byte, gcm.NonceSize())
    if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
        return "", fmt.Errorf("failed to generate nonce: %w", err)
    }
    
    ciphertext := gcm.Seal(nonce, nonce, []byte(plaintext), nil)
    return base64.StdEncoding.EncodeToString(ciphertext), nil
}

// decryptSecret decrypts a string using AES-256-GCM
func decryptSecret(encrypted string, key []byte) (string, error) {
    data, err := base64.StdEncoding.DecodeString(encrypted)
    if err != nil {
        return "", fmt.Errorf("failed to decode base64: %w", err)
    }
    
    block, err := aes.NewCipher(key)
    if err != nil {
        return "", fmt.Errorf("failed to create cipher: %w", err)
    }
    
    gcm, err := cipher.NewGCM(block)
    if err != nil {
        return "", fmt.Errorf("failed to create GCM: %w", err)
    }
    
    if len(data) < gcm.NonceSize() {
        return "", errors.New("ciphertext too short")
    }
    
    plaintext, err := gcm.Open(nil, data[:gcm.NonceSize()], data[gcm.NonceSize():], nil)
    if err != nil {
        return "", fmt.Errorf("failed to decrypt: %w", err)
    }
    
    return string(plaintext), nil
}
```

**Testing**:
- Unit tests for encryption/decryption round-trip
- Test with invalid keys (should fail gracefully)
- Test with tampered ciphertext (should detect authentication failure)

### 1.2 Enhanced Error Wrapping (Go)

**File**: All Go files, starting with `config.go` and `rpc/server.go`

**Changes Required**:
- Wrap every error with context using `fmt.Errorf("context: %w", err)`
- Create custom error types for domain-specific errors
- Add error classification (user error, system error, transient error)

**Implementation Example**:
```go
// Before:
return nil, fmt.Errorf("failed to read config file: %w", err)

// After (with error classification):
if os.IsNotExist(err) {
    return nil, &ConfigNotFoundError{Path: path}
}
if os.IsPermission(err) {
    return nil, &ConfigPermissionError{Path: path}
}
return nil, fmt.Errorf("config load error (path=%s): %w", path, err)
```

**Custom Error Types**:
```go
type ConfigNotFoundError struct {
    Path string
}

func (e *ConfigNotFoundError) Error() string {
    return fmt.Sprintf("configuration file not found: %s", e.Path)
}

func (e *ConfigNotFoundError) Is(target error) bool {
    _, ok := target.(*ConfigNotFoundError)
    return ok
}

type RetryableError struct {
    Err error
}

func (e *RetryableError) Error() string {
    return fmt.Sprintf("retryable error: %v", e.Err)
}

func (e *RetryableError) Unwrap() error {
    return e.Err
}
```

### 1.3 Structured Panic Recovery (Go)

**File**: `pocketd/internal/rpc/server.go`

**Changes Required**:
- Log panic with full stack trace
- Include method name, params (sanitized), and connection info
- Return structured error to client without leaking internals
- Track panic count for circuit breaker pattern

**Implementation**:
```go
// handleMethod with enhanced panic recovery
func (s *Server) handleMethod(ctx context.Context, method string, params json.RawMessage) (result interface{}, retErr error) {
    start := time.Now()
    
    // Deferred recovery with structured logging
    defer func() {
        if r := recover(); r != nil {
            // Get stack trace
            buf := make([]byte, 4096)
            n := runtime.Stack(buf, false)
            
            // Log structured panic information
            log.Error("panic in RPC handler",
                "method", method,
                "panic_value", sanitizeValue(r),
                "stack_trace", string(buf[:n]),
                "duration_ms", time.Since(start).Milliseconds(),
            )
            
            // Increment panic counter for circuit breaker
            s.panicCount.Inc()
            
            // Return generic error to client (no internal details leaked)
            retErr = &Error{
                Code:    InternalError,
                Message: "Internal server error occurred",
                Data: map[string]interface{}{
                    "request_id": uuid.New().String(),
                    "timestamp":  time.Now().UTC().Format(time.RFC3339),
                },
            }
        }
    }()
    
    // Validate params size before processing
    if len(params) > MaxParamsSize {
        return nil, &Error{
            Code:    InvalidParams,
            Message: fmt.Sprintf("Params too large: %d bytes (max: %d)", len(params), MaxParamsSize),
        }
    }
    
    handler, ok := s.handlers[method]
    if !ok {
        return nil, &Error{
            Code:    MethodNotFound,
            Message: "Method not found: " + method,
        }
    }
    
    return handler(ctx, params)
}

// sanitizeValue converts panic values to safe strings
func sanitizeValue(v interface{}) string {
    switch val := v.(type) {
    case string:
        if len(val) > 100 {
            return val[:100] + "...[truncated]"
        }
        return val
    case error:
        return val.Error()
    default:
        return fmt.Sprintf("%v", val)
    }
}
```

### 1.4 Dart Exception Hierarchy

**File**: `flutter_app/lib/services/pocketd_client.dart`

**Changes Required**:
- Create sealed exception hierarchy using freezed
- Replace generic `Exception` with specific types
- Add retry-after metadata for rate limit errors
- Include error codes for programmatic handling

**Implementation**:
```dart
import 'package:freezed_annotation/freezed_annotation.dart';

part 'pocketd_exceptions.freezed.dart';

@freezed
class PocketdException with _$PocketdException implements Exception {
  const factory PocketdException.connectionFailed({
    required String socketPath,
    required Object underlyingError,
  }) = ConnectionFailedException;
  
  const factory PocketdException.notConnected() = NotConnectedException;
  
  const factory PocketdException.requestTimeout({
    required String method,
    required Duration timeout,
  }) = RequestTimeoutException;
  
  const factory PocketdException.jsonRpcError({
    required int code,
    required String message,
    Object? data,
  }) = JsonRpcErrorException;
  
  const factory PocketdException.reconnectFailed({
    required int attempts,
    required Object lastError,
  }) = ReconnectFailedException;
  
  const factory PocketdException.securityError({
    required String reason,
  }) = SecurityException;
}

// Usage example with pattern matching:
Future<void> callDaemon() async {
  try {
    await client.daemonStatus();
  } on PocketdException catch (e) {
    e.map(
      connectionFailed: (e) => _showConnectionError(e.socketPath),
      notConnected: (e) => _attemptReconnect(),
      requestTimeout: (e) => _showTimeoutError(e.method),
      jsonRpcError: (e) => _handleRpcError(e.code, e.message),
      reconnectFailed: (e) => _giveUpAndNotifyUser(e.attempts),
      securityError: (e) => _criticalSecurityAlert(e.reason),
    );
  }
}
```

### 1.5 Result Pattern for Dart

**File**: `flutter_app/lib/core/result.dart` (new file)

**Implementation**:
```dart
import 'package:freezed_annotation/freezed_annotation.dart';

part 'result.freezed.dart';

@freezed
class Result<T, E extends Exception> with _$Result<T, E> {
  const factory Result.success(T value) = Success<T, E>;
  const factory Result.failure(E error) = Failure<T, E>;
  
  /// Convenience constructor for operations that might fail
  static Result<T, Exception> guard<T>(T Function() operation) {
    try {
      return Result.success(operation());
    } on Exception catch (e) {
      return Result.failure(e);
    }
  }
}

// Extension methods for easier handling
extension ResultExtensions<T, E extends Exception> on Result<T, E> {
  T get valueOrThrow {
    return when(
      success: (value) => value,
      failure: (error) => throw error,
    );
  }
  
  T? get valueOrNull {
    return when(
      success: (value) => value,
      failure: (_) => null,
    );
  }
  
  Future<Result<U, E>> bindAsync<U>(Future<Result<U, E>> Function(T) fn) async {
    return when(
      success: (value) => fn(value),
      failure: (error) => Future.value(Result.failure(error)),
    );
  }
}

// Usage in providers:
@riverpod
Future<Result<DaemonStatus, PocketdException>> daemonStatus(DaemonStatusRef ref) async {
  final client = ref.watch(pocketdClientProvider);
  
  return Result.guard(() async {
    final response = await client.daemonStatus();
    return DaemonStatus.fromJson(response);
  }).recoverWith((error) async {
    if (error is ConnectionFailedException) {
      await client.reconnect();
      return Result.guard(() => client.daemonStatus());
    }
    return Result.failure(error);
  });
}
```

## Phase 2: Medium-Priority Enhancements

### 2.1 Request Size Limits (Go RPC)

**File**: `pocketd/internal/rpc/server.go`

Add constants and validation:
```go
const (
    MaxRequestSize = 1 << 20 // 1MB
    MaxParamsSize = 512 << 10 // 512KB
    MaxStringLength = 10 << 10 // 10KB
)

// In handleConnection:
conn.SetReadDeadline(time.Now().Add(30 * time.Second))
limitedReader := io.LimitReader(reader, MaxRequestSize)
decoder := json.NewDecoder(limitedReader)
```

### 2.2 Secure Memory Zeroing (Go)

**File**: `pocketd/internal/security/secure_memory.go` (new file)

```go
package security

import (
    "syscall"
)

// SecureBytes wraps a byte slice that will be zeroed on release
type SecureBytes struct {
    data []byte
}

func NewSecureBytes(size int) *SecureBytes {
    return &SecureBytes{
        data: make([]byte, size),
    }
}

func (s *SecureBytes) Data() []byte {
    return s.data
}

func (s *SecureBytes) Release() {
    // Zero out memory before releasing
    for i := range s.data {
        s.data[i] = 0
    }
    s.data = nil
    
    // Force garbage collection
    syscall.GC()
}

// Use for sensitive data:
secret := NewSecureBytes(64)
defer secret.Release()
// ... use secret.Data() ...
```

### 2.3 Credential Storage in Flutter

**File**: `flutter_app/lib/services/secure_storage.dart` (new file)

```dart
import 'package:encrypted_shared_preferences/encrypted_shared_preferences.dart';
import 'package:flutter_secure_storage/flutter_secure_storage.dart';

class SecureCredentialStorage {
  final FlutterSecureStorage _secureStorage;
  final EncryptedSharedPreferences? _encryptedPrefs;
  
  SecureCredentialStorage()
      : _secureStorage = const FlutterSecureStorage(
          aOptions: AndroidOptions(
            encryptedSharedPreferences: true,
            preferencesKeyPrefix: 'pocketserver_',
          ),
          iOptions: IOSOptions(
            accessibility: KeychainAccessibility.first_unlock_this_device,
          ),
        );
  
  Future<void> storeApiKey(String keyId, String apiKey) async {
    await _secureStorage.write(
      key: 'api_key:$keyId',
      value: apiKey,
    );
  }
  
  Future<String?> getApiKey(String keyId) async {
    return await _secureStorage.read(key: 'api_key:$keyId');
  }
  
  Future<void> deleteApiKey(String keyId) async {
    await _secureStorage.delete(key: 'api_key:$keyId');
  }
  
  // For config.json security fields, use Android Keystore directly
  Future<void> storeConfigSecret(String fieldName, String value) async {
    await _secureStorage.write(
      key: 'config_secret:$fieldName',
      value: value,
    );
  }
}
```

## Phase 3: Testing & Validation

### 3.1 Error Handling Tests (Go)

**File**: `pocketd/internal/config/config_error_test.go`

```go
func TestLoadErrors(t *testing.T) {
    t.Run("file not found returns ConfigNotFoundError", func(t *testing.T) {
        _, err := Load("/nonexistent/path/config.json")
        
        var notFound *ConfigNotFoundError
        if !errors.As(err, &notFound) {
            t.Fatalf("expected ConfigNotFoundError, got %T: %v", err, err)
        }
        
        if notFound.Path != "/nonexistent/path/config.json" {
            t.Errorf("wrong path in error: %s", notFound.Path)
        }
    })
    
    t.Run("permission denied returns ConfigPermissionError", func(t *testing.T) {
        // Create file with no read permissions
        // ... test setup ...
        
        _, err := Load(path)
        
        var permErr *ConfigPermissionError
        if !errors.As(err, &permErr) {
            t.Fatalf("expected ConfigPermissionError, got %T: %v", err, err)
        }
    })
}
```

### 3.2 Panic Recovery Tests (Go)

**File**: `pocketd/internal/rpc/panic_test.go`

```go
func TestPanicRecovery(t *testing.T) {
    server := setupTestServer()
    
    // Register a handler that panics
    server.Register("test.panic", func(ctx context.Context, params json.RawMessage) (interface{}, error) {
        panic("intentional panic for testing")
    })
    
    // Call the panicking handler
    result, err := server.handleMethod(context.Background(), "test.panic", nil)
    
    // Should not crash, should return structured error
    if result != nil {
        t.Errorf("expected nil result, got %v", result)
    }
    
    rpcErr, ok := err.(*Error)
    if !ok {
        t.Fatalf("expected RPC Error, got %T: %v", err, err)
    }
    
    if rpcErr.Code != InternalError {
        t.Errorf("expected code %d, got %d", InternalError, rpcErr.Code)
    }
    
    if rpcErr.Message == "intentional panic for testing" {
        t.Error("panic message leaked to client")
    }
}
```

### 3.3 Exception Hierarchy Tests (Dart)

**File**: `flutter_app/test/pocketd_exceptions_test.dart`

```dart
void main() {
  group('PocketdException', () {
    test('connectionFailed exception contains socket path', () {
      const exception = PocketdException.connectionFailed(
        socketPath: '/tmp/test.sock',
        underlyingError: 'Connection refused',
      );
      
      expect(exception.socketPath, equals('/tmp/test.sock'));
      expect(exception.maybeMap(
        connectionFailed: (e) => true,
        orElse: () => false,
      ), isTrue);
    });
    
    test('jsonRpcError preserves error code and message', () {
      const exception = PocketdException.jsonRpcError(
        code: -32600,
        message: 'Invalid Request',
        data: {'details': 'extra info'},
      );
      
      expect(exception.code, equals(-32600));
      expect(exception.message, equals('Invalid Request'));
      expect(exception.data, equals({'details': 'extra info'}));
    });
  });
  
  group('Result pattern', () {
    test('guard catches exceptions', () {
      final result = Result<int, Exception>.guard(() {
        throw Exception('test error');
      });
      
      expect(result, isA<Failure<int, Exception>>());
      expect(result.valueOrNull, isNull);
    });
    
    test('bindAsync chains operations', () async {
      final result = Result.success(5);
      
      final chained = await result.bindAsync((value) async {
        return Result.success(value * 2);
      });
      
      expect(chained.valueOrThrow, equals(10));
    });
  });
}
```

## Implementation Checklist

### Week 1: Critical Security Fixes
- [ ] 1.1 Encrypt sensitive config fields (Go)
- [ ] 1.2 Enhanced error wrapping across all Go packages
- [ ] 1.3 Structured panic recovery in RPC server
- [ ] 1.4 Dart exception hierarchy with freezed
- [ ] 1.5 Result pattern implementation in Flutter

### Week 2: Medium Priority
- [ ] 2.1 Request size limits in RPC server
- [ ] 2.2 Secure memory zeroing for sensitive data
- [ ] 2.3 Credential storage service in Flutter
- [ ] Error handling tests for all new code
- [ ] Integration tests for panic recovery

### Week 3: Validation & Documentation
- [ ] 3.1 Comprehensive error handling tests
- [ ] 3.2 Panic recovery stress tests
- [ ] 3.3 Exception hierarchy tests
- [ ] Update SECURITY.md with new protections
- [ ] Document error codes and recovery procedures

## Success Metrics

1. **Zero plaintext secrets** in config.json (verified by hexdump)
2. **All errors wrapped** with context (verified by grep for `fmt.Errorf` without `%w`)
3. **No panic crashes** in 1000-request fuzz test
4. **Type-safe error handling** in 100% of Dart code (verified by analyzer)
5. **< 1ms overhead** added by encryption/decryption (benchmarked)

## References

- Go Error Wrapping Best Practices: https://go.dev/blog/error-handling-and-go
- Dart Sealed Classes: https://dart.dev/language/class-modifiers
- OWASP Mobile Security Testing Guide: https://owasp.org/www-project-mobile-security-testing-guide/
- Android Keystore System: https://developer.android.com/training/articles/keystore
