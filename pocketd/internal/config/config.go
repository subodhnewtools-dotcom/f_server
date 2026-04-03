// Package config provides configuration management for pocketd.
// It handles loading, validating, and saving the config.json file
// with atomic writes to prevent corruption.
package config

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
)

// Config represents the complete pocketd configuration structure.
// All fields must conform to the canonical schema defined in the project spec.
type Config struct {
	Version      string      `json:"version"`
	DeviceID     string      `json:"device_id"`
	DeviceName   string      `json:"device_name"`
	Resources    Resources   `json:"resources"`
	Stack        Stack       `json:"stack"`
	Network      Network     `json:"network"`
	Replication  Replication `json:"replication"`
	Backup       Backup      `json:"backup"`
	Security     Security    `json:"security"`
}

// Resources defines resource allocation limits for the server stack.
type Resources struct {
	RAMMB     int    `json:"ram_mb"`
	StorageMB int    `json:"storage_mb"`
	CPUPercent int   `json:"cpu_percent"`
	Ports     Ports  `json:"ports"`
}

// Ports defines the port numbers for each service.
type Ports struct {
	HTTP         int `json:"http"`
	HTTPS        int `json:"https"`
	MySQL        int `json:"mysql"`
	Redis        int `json:"redis"`
	HAProxyStats int `json:"haproxy_stats"`
}

// Stack defines which services are enabled in the server stack.
type Stack struct {
	PHP     bool `json:"php"`
	NodeJS  bool `json:"nodejs"`
	Redis   bool `json:"redis"`
	Python  bool `json:"python"`
}

// Network defines network configuration including Cloudflare tunnel and peer relay.
type Network struct {
	CloudflareTunnelToken *string `json:"cloudflare_tunnel_token"`
	BindLocalhostOnly     bool    `json:"bind_localhost_only"`
	PeerRelayURL          *string `json:"peer_relay_url"`
}

// Replication defines replication settings for multi-node sync.
type Replication struct {
	Mode            string `json:"mode"`
	SyncIntervalMS  int    `json:"sync_interval_ms"`
	Peers           []Peer `json:"peers"`
}

// Peer represents a peer node in the replication mesh.
type Peer struct {
	PeerID   string `json:"peer_id"`
	Name     string `json:"name"`
	Address  string `json:"address"`
	CertPEM  string `json:"cert_pem"`
}

// Backup defines backup schedule and destinations.
type Backup struct {
	Schedule       string        `json:"schedule"`
	RetentionDays  int           `json:"retention_days"`
	Destinations   []Destination `json:"destinations"`
}

// Destination represents a backup destination configuration.
type Destination struct {
	ID        string                 `json:"id"`
	Type      string                 `json:"type"` // local, sftp, s3, gdrive, peer
	Config    map[string]interface{} `json:"config"`
}

// Security holds security-related configuration including secrets.
type Security struct {
	APIKeySalt string `json:"api_key_salt"`
	JWTSecret  string `json:"jwt_secret"`
}

// Validator provides configuration validation.
type Validator struct {
	config *Config
	errors []error
}

// Load reads and validates the configuration from the specified path.
// Returns an error if the file doesn't exist, is malformed, or fails validation.
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("config file not found: %s", path)
		}
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	config, err := Parse(data)
	if err != nil {
		return nil, err
	}

	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("config validation failed: %w", err)
	}

	return config, nil
}

// Parse parses configuration from JSON bytes.
func Parse(data []byte) (*Config, error) {
	var config Config
	
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	
	if err := decoder.Decode(&config); err != nil {
		return nil, fmt.Errorf("failed to parse config JSON: %w", err)
	}

	return &config, nil
}

// Save writes the configuration to the specified path using atomic write.
// It writes to a temporary file first, then renames to prevent corruption.
func (c *Config) Save(path string) error {
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	// Ensure parent directory exists
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	// Write to temporary file
	tmpPath := path + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0600); err != nil {
		return fmt.Errorf("failed to write temp config file: %w", err)
	}

	// Atomic rename
	if err := os.Rename(tmpPath, path); err != nil {
		os.Remove(tmpPath) // Clean up on failure
		return fmt.Errorf("failed to rename config file: %w", err)
	}

	return nil
}

// Validate performs comprehensive validation of the configuration.
// Returns nil if valid, or an error describing the validation failures.
func (c *Config) Validate() error {
	v := &Validator{config: c}
	v.validateVersion()
	v.validateDeviceID()
	v.validateDeviceName()
	v.validateResources()
	v.validateStack()
	v.validateNetwork()
	v.validateReplication()
	v.validateBackup()
	v.validateSecurity()

	if len(v.errors) > 0 {
		return &ValidationError{Errors: v.errors}
	}
	return nil
}

// validateVersion checks the version field.
func (v *Validator) validateVersion() {
	if v.config.Version == "" {
		v.errors = append(v.errors, fmt.Errorf("version is required"))
		return
	}
	
	// Basic semver check
	matched, _ := regexp.MatchString(`^\d+\.\d+$`, v.config.Version)
	if !matched {
		v.errors = append(v.errors, fmt.Errorf("version must be in format X.Y, got: %s", v.config.Version))
	}
}

// validateDeviceID checks the device ID field.
func (v *Validator) validateDeviceID() {
	if v.config.DeviceID == "" {
		v.errors = append(v.errors, fmt.Errorf("device_id is required"))
		return
	}
	
	// UUID v4 format check
	uuidRegex := regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)
	if !uuidRegex.MatchString(v.config.DeviceID) {
		v.errors = append(v.errors, fmt.Errorf("device_id must be a valid UUID v4, got: %s", v.config.DeviceID))
	}
}

// validateDeviceName checks the device name field.
func (v *Validator) validateDeviceName() {
	if v.config.DeviceName == "" {
		v.errors = append(v.errors, fmt.Errorf("device_name is required"))
		return
	}
	
	if len(v.config.DeviceName) > 100 {
		v.errors = append(v.errors, fmt.Errorf("device_name must be <= 100 characters"))
	}
}

// validateResources checks resource allocation settings.
func (v *Validator) validateResources() {
	r := v.config.Resources
	
	// RAM: minimum 128MB, maximum based on typical Android device
	if r.RAMMB < 128 {
		v.errors = append(v.errors, fmt.Errorf("ram_mb must be >= 128, got: %d", r.RAMMB))
	}
	if r.RAMMB > 8192 {
		v.errors = append(v.errors, fmt.Errorf("ram_mb must be <= 8192, got: %d", r.RAMMB))
	}
	
	// Storage: minimum 512MB
	if r.StorageMB < 512 {
		v.errors = append(v.errors, fmt.Errorf("storage_mb must be >= 512, got: %d", r.StorageMB))
	}
	
	// CPU percent: 1-100
	if r.CPUPercent < 1 || r.CPUPercent > 100 {
		v.errors = append(v.errors, fmt.Errorf("cpu_percent must be 1-100, got: %d", r.CPUPercent))
	}
	
	// Validate ports
	v.validatePorts()
}

// validatePorts checks port number assignments.
func (v *Validator) validatePorts() {
	p := v.config.Resources.Ports
	
	// Port range: 1024-65535 (non-privileged ports)
	portValidator := func(name string, port int) {
		if port < 1024 || port > 65535 {
			v.errors = append(v.errors, fmt.Errorf("%s port must be 1024-65535, got: %d", name, port))
		}
	}
	
	portValidator("http", p.HTTP)
	portValidator("https", p.HTTPS)
	portValidator("mysql", p.MySQL)
	portValidator("redis", p.Redis)
	portValidator("haproxy_stats", p.HAProxyStats)
	
	// Check for duplicate ports - use slice for deterministic ordering
	type portEntry struct {
		name string
		port int
	}
	
	ports := []portEntry{
		{"http", p.HTTP},
		{"https", p.HTTPS},
		{"mysql", p.MySQL},
		{"redis", p.Redis},
		{"haproxy_stats", p.HAProxyStats},
	}
	
	seen := make(map[int]string)
	for _, entry := range ports {
		if existing, ok := seen[entry.port]; ok {
			v.errors = append(v.errors, fmt.Errorf("duplicate port %d used by %s and %s", entry.port, existing, entry.name))
		}
		seen[entry.port] = entry.name
	}
}

// validateStack checks stack configuration.
func (v *Validator) validateStack() {
	s := v.config.Stack
	
	// At least one service must be enabled
	if !s.PHP && !s.NodeJS && !s.Redis && !s.Python {
		v.errors = append(v.errors, fmt.Errorf("at least one stack service must be enabled"))
	}
}

// validateNetwork checks network configuration.
func (v *Validator) validateNetwork() {
	n := v.config.Network
	
	// If Cloudflare token is provided, it must be non-empty
	if n.CloudflareTunnelToken != nil && *n.CloudflareTunnelToken == "" {
		v.errors = append(v.errors, fmt.Errorf("cloudflare_tunnel_token cannot be empty string"))
	}
	
	// If peer relay URL is provided, validate format
	if n.PeerRelayURL != nil {
		urlRegex := regexp.MustCompile(`^https?://[a-zA-Z0-9.-]+(:\d+)?(/.*)?$`)
		if !urlRegex.MatchString(*n.PeerRelayURL) {
			v.errors = append(v.errors, fmt.Errorf("peer_relay_url must be a valid HTTP(S) URL"))
		}
	}
}

// validateReplication checks replication configuration.
func (v *Validator) validateReplication() {
	r := v.config.Replication
	
	// Mode must be "async" or "sync"
	if r.Mode != "async" && r.Mode != "sync" {
		v.errors = append(v.errors, fmt.Errorf("replication mode must be 'async' or 'sync', got: %s", r.Mode))
	}
	
	// Sync interval: 100ms - 60000ms
	if r.SyncIntervalMS < 100 || r.SyncIntervalMS > 60000 {
		v.errors = append(v.errors, fmt.Errorf("sync_interval_ms must be 100-60000, got: %d", r.SyncIntervalMS))
	}
	
	// Validate peers
	for i, peer := range r.Peers {
		if peer.PeerID == "" {
			v.errors = append(v.errors, fmt.Errorf("peer[%d].peer_id is required", i))
		}
		if peer.Name == "" {
			v.errors = append(v.errors, fmt.Errorf("peer[%d].name is required", i))
		}
		if peer.Address == "" {
			v.errors = append(v.errors, fmt.Errorf("peer[%d].address is required", i))
		}
	}
}

// validateBackup checks backup configuration.
func (v *Validator) validateBackup() {
	b := v.config.Backup
	
	// Schedule must be valid
	validSchedules := map[string]bool{
		"hourly": true,
		"daily": true,
		"weekly": true,
		"manual": true,
	}
	
	if !validSchedules[b.Schedule] {
		v.errors = append(v.errors, fmt.Errorf("backup schedule must be hourly/daily/weekly/manual, got: %s", b.Schedule))
	}
	
	// Retention: 1-365 days
	if b.RetentionDays < 1 || b.RetentionDays > 365 {
		v.errors = append(v.errors, fmt.Errorf("retention_days must be 1-365, got: %d", b.RetentionDays))
	}
	
	// Validate destinations
	for i, dest := range b.Destinations {
		if dest.ID == "" {
			v.errors = append(v.errors, fmt.Errorf("destination[%d].id is required", i))
		}
		if dest.Type == "" {
			v.errors = append(v.errors, fmt.Errorf("destination[%d].type is required", i))
		}
		
		validTypes := map[string]bool{"local": true, "sftp": true, "s3": true, "gdrive": true, "peer": true}
		if !validTypes[dest.Type] {
			v.errors = append(v.errors, fmt.Errorf("destination[%d].type must be local/sftp/s3/gdrive/peer", i))
		}
	}
}

// validateSecurity checks security configuration.
func (v *Validator) validateSecurity() {
	s := v.config.Security
	
	// API key salt must be 64 hex characters (32 bytes)
	if len(s.APIKeySalt) != 64 {
		v.errors = append(v.errors, fmt.Errorf("api_key_salt must be 64 hex characters, got: %d", len(s.APIKeySalt)))
	}
	
	// JWT secret must be 64 hex characters (32 bytes)
	if len(s.JWTSecret) != 64 {
		v.errors = append(v.errors, fmt.Errorf("jwt_secret must be 64 hex characters, got: %d", len(s.JWTSecret)))
	}
	
	// Verify they are valid hex
	hexRegex := regexp.MustCompile(`^[0-9a-fA-F]+$`)
	if !hexRegex.MatchString(s.APIKeySalt) {
		v.errors = append(v.errors, fmt.Errorf("api_key_salt must be valid hexadecimal"))
	}
	if !hexRegex.MatchString(s.JWTSecret) {
		v.errors = append(v.errors, fmt.Errorf("jwt_secret must be valid hexadecimal"))
	}
}

// ValidationError represents one or more configuration validation errors.
type ValidationError struct {
	Errors []error
}

// Error implements the error interface.
func (e *ValidationError) Error() string {
	if len(e.Errors) == 1 {
		return e.Errors[0].Error()
	}
	
	msg := fmt.Sprintf("%d validation errors:", len(e.Errors))
	for _, err := range e.Errors {
		msg += "\n  - " + err.Error()
	}
	return msg
}

// Default returns a configuration with sensible defaults.
// This is used when creating a new configuration file.
func Default() *Config {
	return &Config{
		Version:    "1.0",
		DeviceName: "PocketServer Device",
		Resources: Resources{
			RAMMB:     512,
			StorageMB: 5120,
			CPUPercent: 30,
			Ports: Ports{
				HTTP:         8080,
				HTTPS:        8443,
				MySQL:        3306,
				Redis:        6379,
				HAProxyStats: 9000,
			},
		},
		Stack: Stack{
			PHP:    true,
			NodeJS: false,
			Redis:  false,
			Python: false,
		},
		Network: Network{
			BindLocalhostOnly: true,
		},
		Replication: Replication{
			Mode:           "async",
			SyncIntervalMS: 500,
			Peers:          []Peer{},
		},
		Backup: Backup{
			Schedule:      "daily",
			RetentionDays: 30,
			Destinations:  []Destination{},
		},
	}
}
