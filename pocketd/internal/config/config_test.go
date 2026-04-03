package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestLoadValidConfig tests loading a valid configuration file.
func TestLoadValidConfig(t *testing.T) {
	// Create a temporary directory for test files
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.json")
	
	// Valid config JSON
	validConfig := `{
		"version": "1.0",
		"device_id": "550e8400-e29b-41d4-a716-446655440000",
		"device_name": "Test Device",
		"resources": {
			"ram_mb": 512,
			"storage_mb": 5120,
			"cpu_percent": 30,
			"ports": {
				"http": 8080,
				"https": 8443,
				"mysql": 3306,
				"redis": 6379,
				"haproxy_stats": 9000
			}
		},
		"stack": {
			"php": true,
			"nodejs": false,
			"redis": false,
			"python": false
		},
		"network": {
			"cloudflare_tunnel_token": null,
			"bind_localhost_only": true,
			"peer_relay_url": null
		},
		"replication": {
			"mode": "async",
			"sync_interval_ms": 500,
			"peers": []
		},
		"backup": {
			"schedule": "daily",
			"retention_days": 30,
			"destinations": []
		},
		"security": {
			"api_key_salt": "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
			"jwt_secret": "fedcba9876543210fedcba9876543210fedcba9876543210fedcba9876543210"
		}
	}`
	
	// Write config file
	if err := os.WriteFile(configPath, []byte(validConfig), 0600); err != nil {
		t.Fatalf("Failed to write test config: %v", err)
	}
	
	// Load and validate
	config, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load() returned error: %v", err)
	}
	
	// Verify fields
	if config.Version != "1.0" {
		t.Errorf("Expected version '1.0', got '%s'", config.Version)
	}
	if config.DeviceID != "550e8400-e29b-41d4-a716-446655440000" {
		t.Errorf("Expected device_id '550e8400-e29b-41d4-a716-446655440000', got '%s'", config.DeviceID)
	}
	if config.Resources.RAMMB != 512 {
		t.Errorf("Expected ram_mb 512, got %d", config.Resources.RAMMB)
	}
	if !config.Stack.PHP {
		t.Error("Expected PHP stack to be enabled")
	}
}

// TestLoadFileNotFound tests that Load returns appropriate error when file doesn't exist.
func TestLoadFileNotFound(t *testing.T) {
	_, err := Load("/nonexistent/path/config.json")
	if err == nil {
		t.Fatal("Load() should return error for non-existent file")
	}
	
	if !strings.Contains(err.Error(), "config file not found") {
		t.Errorf("Expected 'config file not found' error, got: %v", err)
	}
}

// TestLoadInvalidJSON tests that Load returns error for malformed JSON.
func TestLoadInvalidJSON(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.json")
	
	invalidJSON := `{ invalid json }`
	
	if err := os.WriteFile(configPath, []byte(invalidJSON), 0600); err != nil {
		t.Fatalf("Failed to write test config: %v", err)
	}
	
	_, err := Load(configPath)
	if err == nil {
		t.Fatal("Load() should return error for invalid JSON")
	}
	
	if !strings.Contains(err.Error(), "failed to parse config JSON") {
		t.Errorf("Expected parse error, got: %v", err)
	}
}

// TestValidateMinRAM tests RAM minimum validation.
func TestValidateMinRAM(t *testing.T) {
	config := Default()
	config.Resources.RAMMB = 64 // Below minimum
	
	err := config.Validate()
	if err == nil {
		t.Fatal("Validate() should return error for RAM < 128MB")
	}
	
	if !strings.Contains(err.Error(), "ram_mb must be >= 128") {
		t.Errorf("Expected RAM validation error, got: %v", err)
	}
}

// TestValidatePortRange tests port range validation.
func TestValidatePortRange(t *testing.T) {
	config := Default()
	config.Resources.Ports.HTTP = 80 // Privileged port
	
	err := config.Validate()
	if err == nil {
		t.Fatal("Validate() should return error for privileged port")
	}
	
	if !strings.Contains(err.Error(), "http port must be 1024-65535") {
		t.Errorf("Expected port validation error, got: %v", err)
	}
}

// TestValidateDuplicatePorts tests duplicate port detection.
func TestValidateDuplicatePorts(t *testing.T) {
	config := Default()
	config.DeviceID = "550e8400-e29b-41d4-a716-446655440000"
	config.Security.APIKeySalt = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	config.Security.JWTSecret = "fedcba9876543210fedcba9876543210fedcba9876543210fedcba9876543210"
	config.Resources.Ports.HTTP = 8080
	config.Resources.Ports.HTTPS = 8080 // Duplicate
	
	err := config.Validate()
	if err == nil {
		t.Fatal("Validate() should return error for duplicate ports")
	}
	
	if !strings.Contains(err.Error(), "duplicate port") {
		t.Errorf("Expected duplicate port error, got: %v", err)
	}
}

// TestValidateDeviceIDFormat tests UUID v4 format validation.
func TestValidateDeviceIDFormat(t *testing.T) {
	tests := []struct {
		name      string
		deviceID  string
		wantError bool
	}{
		{"valid uuid", "550e8400-e29b-41d4-a716-446655440000", false},
		{"invalid format", "not-a-uuid", true},
		{"empty", "", true},
		{"wrong version", "550e8400-e29b-31d4-a716-446655440000", true}, // version 3, not 4
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := Default()
			config.DeviceID = tt.deviceID
			config.Security.APIKeySalt = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
			config.Security.JWTSecret = "fedcba9876543210fedcba9876543210fedcba9876543210fedcba9876543210"
			
			err := config.Validate()
			if tt.wantError && err == nil {
				t.Errorf("Expected validation error for device_id '%s'", tt.deviceID)
			}
			if !tt.wantError && err != nil {
				t.Errorf("Unexpected validation error for device_id '%s': %v", tt.deviceID, err)
			}
		})
	}
}

// TestValidateSecurityFields tests security field validation.
func TestValidateSecurityFields(t *testing.T) {
	tests := []struct {
		name       string
		apiKeySalt string
		jwtSecret  string
		wantError  bool
	}{
		{
			"valid hex 64 chars",
			"0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
			"fedcba9876543210fedcba9876543210fedcba9876543210fedcba9876543210",
			false,
		},
		{
			"too short",
			"0123456789abcdef",
			"fedcba9876543210fedcba9876543210fedcba9876543210fedcba9876543210",
			true,
		},
		{
			"invalid hex",
			"zzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzz",
			"fedcba9876543210fedcba9876543210fedcba9876543210fedcba9876543210",
			true,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := Default()
			config.DeviceID = "550e8400-e29b-41d4-a716-446655440000"
			config.Security.APIKeySalt = tt.apiKeySalt
			config.Security.JWTSecret = tt.jwtSecret
			
			err := config.Validate()
			if tt.wantError && err == nil {
				t.Error("Expected validation error for invalid security fields")
			}
			if !tt.wantError && err != nil {
				t.Errorf("Unexpected validation error: %v", err)
			}
		})
	}
}

// TestSaveAtomicWrite tests that Save uses atomic write.
func TestSaveAtomicWrite(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.json")
	
	config := Default()
	config.DeviceID = "550e8400-e29b-41d4-a716-446655440000"
	config.Security.APIKeySalt = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	config.Security.JWTSecret = "fedcba9876543210fedcba9876543210fedcba9876543210fedcba9876543210"
	
	err := config.Save(configPath)
	if err != nil {
		t.Fatalf("Save() returned error: %v", err)
	}
	
	// Verify file exists and has correct permissions
	info, err := os.Stat(configPath)
	if err != nil {
		t.Fatalf("Config file not created: %v", err)
	}
	
	// Check permissions (should be 0600)
	if info.Mode().Perm()&0777 != 0600 {
		t.Errorf("Expected file permissions 0600, got %o", info.Mode().Perm()&0777)
	}
	
	// Verify no temp file remains
	tmpPath := configPath + ".tmp"
	if _, err := os.Stat(tmpPath); err == nil {
		t.Error("Temporary file was not cleaned up")
	}
	
	// Verify content can be loaded back
	loaded, err := Load(configPath)
	if err != nil {
		t.Fatalf("Failed to load saved config: %v", err)
	}
	
	if loaded.DeviceID != config.DeviceID {
		t.Errorf("DeviceID mismatch after save/load: expected %s, got %s", config.DeviceID, loaded.DeviceID)
	}
}

// TestValidateReplicationMode tests replication mode validation.
func TestValidateReplicationMode(t *testing.T) {
	tests := []struct {
		mode      string
		wantError bool
	}{
		{"async", false},
		{"sync", false},
		{"invalid", true},
		{"", true},
	}
	
	for _, tt := range tests {
		t.Run(tt.mode, func(t *testing.T) {
			config := Default()
			config.DeviceID = "550e8400-e29b-41d4-a716-446655440000"
			config.Replication.Mode = tt.mode
			config.Security.APIKeySalt = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
			config.Security.JWTSecret = "fedcba9876543210fedcba9876543210fedcba9876543210fedcba9876543210"
			
			err := config.Validate()
			if tt.wantError && err == nil {
				t.Errorf("Expected validation error for mode '%s'", tt.mode)
			}
			if !tt.wantError && err != nil {
				t.Errorf("Unexpected validation error for mode '%s': %v", tt.mode, err)
			}
		})
	}
}

// TestValidateBackupSchedule tests backup schedule validation.
func TestValidateBackupSchedule(t *testing.T) {
	tests := []struct {
		schedule  string
		wantError bool
	}{
		{"hourly", false},
		{"daily", false},
		{"weekly", false},
		{"manual", false},
		{"invalid", true},
		{"", true},
	}
	
	for _, tt := range tests {
		t.Run(tt.schedule, func(t *testing.T) {
			config := Default()
			config.DeviceID = "550e8400-e29b-41d4-a716-446655440000"
			config.Backup.Schedule = tt.schedule
			config.Security.APIKeySalt = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
			config.Security.JWTSecret = "fedcba9876543210fedcba9876543210fedcba9876543210fedcba9876543210"
			
			err := config.Validate()
			if tt.wantError && err == nil {
				t.Errorf("Expected validation error for schedule '%s'", tt.schedule)
			}
			if !tt.wantError && err != nil {
				t.Errorf("Unexpected validation error for schedule '%s': %v", tt.schedule, err)
			}
		})
	}
}

// TestDefaultConfig tests that Default() returns a valid configuration.
func TestDefaultConfig(t *testing.T) {
	config := Default()
	config.DeviceID = "550e8400-e29b-41d4-a716-446655440000"
	config.Security.APIKeySalt = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	config.Security.JWTSecret = "fedcba9876543210fedcba9876543210fedcba9876543210fedcba9876543210"
	
	err := config.Validate()
	if err != nil {
		t.Errorf("Default config failed validation: %v", err)
	}
	
	// Verify default values
	if config.Version != "1.0" {
		t.Errorf("Expected default version '1.0', got '%s'", config.Version)
	}
	if config.Resources.RAMMB != 512 {
		t.Errorf("Expected default ram_mb 512, got %d", config.Resources.RAMMB)
	}
	if config.Replication.Mode != "async" {
		t.Errorf("Expected default replication mode 'async', got '%s'", config.Replication.Mode)
	}
	if config.Backup.Schedule != "daily" {
		t.Errorf("Expected default backup schedule 'daily', got '%s'", config.Backup.Schedule)
	}
}

// TestParseDisallowUnknownFields tests that Parse rejects unknown fields.
func TestParseDisallowUnknownFields(t *testing.T) {
	jsonWithUnknown := `{
		"version": "1.0",
		"device_id": "550e8400-e29b-41d4-a716-446655440000",
		"device_name": "Test",
		"unknown_field": "should fail",
		"resources": {
			"ram_mb": 512,
			"storage_mb": 5120,
			"cpu_percent": 30,
			"ports": {
				"http": 8080,
				"https": 8443,
				"mysql": 3306,
				"redis": 6379,
				"haproxy_stats": 9000
			}
		},
		"stack": {
			"php": true,
			"nodejs": false,
			"redis": false,
			"python": false
		},
		"network": {
			"cloudflare_tunnel_token": null,
			"bind_localhost_only": true,
			"peer_relay_url": null
		},
		"replication": {
			"mode": "async",
			"sync_interval_ms": 500,
			"peers": []
		},
		"backup": {
			"schedule": "daily",
			"retention_days": 30,
			"destinations": []
		},
		"security": {
			"api_key_salt": "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
			"jwt_secret": "fedcba9876543210fedcba9876543210fedcba9876543210fedcba9876543210"
		}
	}`
	
	_, err := Parse([]byte(jsonWithUnknown))
	if err == nil {
		t.Fatal("Parse() should reject unknown fields")
	}
	
	if !strings.Contains(err.Error(), "unknown field") {
		t.Errorf("Expected 'unknown field' error, got: %v", err)
	}
}

// TestValidationErrorMultipleErrors tests that ValidationError aggregates multiple errors.
func TestValidationErrorMultipleErrors(t *testing.T) {
	config := &Config{
		Version:    "", // Missing
		DeviceID:   "", // Missing
		DeviceName: "", // Missing
		Resources: Resources{
			RAMMB:     50,  // Too low
			StorageMB: 100, // Too low
		},
		Security: Security{
			APIKeySalt: "short",
			JWTSecret:  "short",
		},
	}
	
	err := config.Validate()
	if err == nil {
		t.Fatal("Validate() should return multiple errors")
	}
	
	vErr, ok := err.(*ValidationError)
	if !ok {
		t.Fatalf("Expected ValidationError, got %T", err)
	}
	
	if len(vErr.Errors) < 5 {
		t.Errorf("Expected at least 5 errors, got %d", len(vErr.Errors))
	}
	
	// Error message should mention multiple errors
	if !strings.Contains(err.Error(), "validation errors") {
		t.Errorf("Expected error message to mention multiple errors, got: %v", err)
	}
}
