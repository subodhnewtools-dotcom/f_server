# PocketServer Security Audit Report

## Executive Summary

**Audit Date:** 2024
**Scope:** All Go (pocketd) and Dart (Flutter app) source files
**Severity Levels:** Critical, High, Medium, Low

---

## Critical Findings

### C-01: Sensitive Configuration Data Stored in Plaintext

**Location:** `/workspace/pocketd/internal/config/config.go`, `/workspace/flutter_app/lib/services/pocketd_client.dart`

**Issue:** Security secrets (`api_key_salt`, `jwt_secret`) are stored as plaintext in `config.json`. If an attacker gains filesystem access, they can extract these secrets.

**Impact:** 
- Compromised API key validation
- Forged JWT tokens
- Full system impersonation

**Fix Required:**
1. Encrypt sensitive fields before writing to disk
2. Use Android Keystore for key derivation
3. Decrypt only in memory when needed

**Status:** ⚠️ PENDING FIX

---

### C-02: Insufficient Error Context in Go

**Location:** Multiple Go files

**Issue:** Errors are not wrapped with context using `fmt.Errorf("context: %w", err)`. This makes debugging production issues extremely difficult.

**Example:**
```go
// BAD
if err != nil {
    return err
}

// GOOD
if err != nil {
    return fmt.Errorf("loading config from %s: %w", path, err)
}
```

**Impact:** Unable to trace error root cause in production logs

**Status:** ⚠️ PENDING FIX

---

### C-03: Panic Recovery Without Proper Logging

**Location:** `/workspace/pocketd/internal/rpc/server.go:265-269`

**Issue:** Panic recovery only prints to stdout without structured logging or error propagation.

```go
defer func() {
    if r := recover(); r != nil {
        fmt.Printf("Panic in handler %s: %v\n", method, r) // ❌ Unstructured
    }
}()
```

**Impact:** 
- No audit trail for crashes
- Cannot correlate panics with requests
- Missing stack traces

**Status:** ⚠️ PENDING FIX

---

### C-04: Dart Generic Exceptions

**Location:** `/workspace/flutter_app/lib/services/pocketd_client.dart`

**Issue:** Using generic `Exception` instead of typed exceptions makes error handling impossible.

```dart
throw Exception('Not connected') // ❌ Cannot pattern match
```

**Impact:** UI cannot provide specific error messages to users

**Fix Required:** Implement sealed exception hierarchy:
- `PocketdNotConnectedException`
- `PocketdTimeoutException`
- `PocketdRpcException`
- `PocketdConnectionLostException`

**Status:** ⚠️ PENDING FIX

---

## High Severity Findings

### H-01: Missing Input Validation on RPC Params

**Location:** `/workspace/pocketd/internal/rpc/server.go`

**Issue:** RPC handlers don't validate parameter types/sizes before processing.

**Impact:** Potential DoS via oversized payloads or malformed params

**Status:** ⚠️ PENDING FIX

---

### H-02: Sensitive Data in Logs

**Location:** Multiple files

**Issue:** Config values (including secrets) could be logged during debug sessions.

**Status:** ⚠️ PENDING FIX

---

### H-03: No Rate Limiting on RPC Server

**Location:** `/workspace/pocketd/internal/rpc/server.go`

**Issue:** No protection against rapid-fire RPC requests from compromised Flutter app.

**Status:** ⚠️ PENDING FIX

---

### H-04: Dart Stream Error Handling

**Location:** `pocketd_client.dart:70-111`

**Issue:** Stream errors don't propagate properly; silent failures in `_listenToSocket`.

**Status:** ⚠️ PENDING FIX

---

## Medium Severity Findings

### M-01: No Timeout on Database Operations

**Issue:** MariaDB queries have no explicit timeout

**Status:** ⚠️ PENDING FIX

---

### M-02: Missing File Permission Checks

**Issue:** Config file created with 0600 but never verified after creation

**Status:** ⚠️ PENDING FIX

---

### M-03: UUID Generation Not Cryptographically Secure

**Location:** Go code uses standard `math/rand`?

**Status:** ✅ NEEDS VERIFICATION

---

## Low Severity Findings

### L-01: TODO Comments in Production Code

**Location:** `server.go:303`

**Issue:** `// TODO: track supervisor start time`

**Status:** ⚠️ PENDING FIX

---

### L-02: Inconsistent Error Message Format

**Issue:** Some errors use "failed to", others use "cannot", others use "unable to"

**Status:** ⚠️ PENDING FIX

---

## Remediation Plan

### Phase 1: Critical Fixes (Immediate)
1. Implement config field encryption
2. Add error wrapping throughout Go codebase
3. Improve panic recovery with structured logging
4. Create typed exception hierarchy in Dart

### Phase 2: High Priority (Within 1 week)
1. Add RPC parameter validation
2. Implement log sanitization
3. Add rate limiting to RPC server
4. Fix Dart stream error handling

### Phase 3: Hardening (Within 2 weeks)
1. Add database query timeouts
2. Implement file permission auditing
3. Verify cryptographic randomness
4. Remove all TODO comments

---

## Testing Requirements

All fixes must include:
- [ ] Unit tests for error paths
- [ ] Integration tests for exception handling
- [ ] Fuzz testing for input validation
- [ ] Manual verification on physical Android devices

---

## Compliance Checklist

- [x] No hardcoded secrets in source code
- [ ] All errors wrapped with context
- [ ] Structured logging for all errors
- [ ] Typed exceptions in Dart
- [ ] Encrypted storage for sensitive data
- [ ] Input validation on all RPC methods
- [ ] Rate limiting implemented
- [ ] Panic recovery with stack traces
- [ ] No sensitive data in logs
- [ ] File permissions verified

**Current Compliance:** 1/10 (10%)

---

*This audit was performed automatically. All findings must be verified manually before deployment.*
