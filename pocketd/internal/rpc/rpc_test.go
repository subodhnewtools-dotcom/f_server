package rpc

import (
	"context"
	"encoding/json"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/pocketserver/pocketd/internal/config"
	"github.com/pocketserver/pocketd/internal/metrics"
	"github.com/pocketserver/pocketd/internal/supervisor"
)

func TestNewServer(t *testing.T) {
	tmpDir := t.TempDir()
	socketPath := filepath.Join(tmpDir, "test.sock")

	cfg := &config.Config{}
	sup, err := supervisor.New(cfg, tmpDir, tmpDir, tmpDir)
	if err != nil {
		t.Fatalf("failed to create supervisor: %v", err)
	}
	coll := metrics.NewCollector(tmpDir)

	server := NewServer(socketPath, cfg, sup, coll)
	if server == nil {
		t.Fatal("NewServer returned nil")
	}

	if server.socketPath != socketPath {
		t.Errorf("expected socketPath %q, got %q", socketPath, server.socketPath)
	}
}

func TestServerStartStop(t *testing.T) {
	tmpDir := t.TempDir()
	socketPath := filepath.Join(tmpDir, "test.sock")

	cfg := &config.Config{}
	sup, err := supervisor.New(cfg, tmpDir, tmpDir, tmpDir)
	if err != nil {
		t.Fatalf("failed to create supervisor: %v", err)
	}
	coll := metrics.NewCollector(tmpDir)

	server := NewServer(socketPath, cfg, sup, coll)

	if err := server.Start(); err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	// Give server time to start
	time.Sleep(100 * time.Millisecond)

	// Verify socket exists
	if _, err := os.Stat(socketPath); err != nil {
		t.Errorf("socket file should exist: %v", err)
	}

	// Stop the server
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := server.Stop(ctx); err != nil {
		t.Errorf("Stop failed: %v", err)
	}

	// Socket should be removed after stop (or at least listener closed)
	// Note: We don't explicitly remove the socket file in our implementation
}

func TestServerRegister(t *testing.T) {
	tmpDir := t.TempDir()
	socketPath := filepath.Join(tmpDir, "test.sock")

	cfg := &config.Config{}
	sup, err := supervisor.New(cfg, tmpDir, tmpDir, tmpDir)
	if err != nil {
		t.Fatalf("failed to create supervisor: %v", err)
	}
	coll := metrics.NewCollector(tmpDir)

	server := NewServer(socketPath, cfg, sup, coll)

	handler := func(ctx context.Context, params json.RawMessage) (interface{}, error) {
		return map[string]interface{}{"ok": true}, nil
	}

	server.Register("test.method", handler)

	// Verify handler is registered
	server.mu.RLock()
	_, ok := server.handlers["test.method"]
	server.mu.RUnlock()

	if !ok {
		t.Error("handler should be registered")
	}
}

func TestServerConcurrentConnections(t *testing.T) {
	tmpDir := t.TempDir()
	socketPath := filepath.Join(tmpDir, "test.sock")

	cfg := &config.Config{}
	sup, err := supervisor.New(cfg, tmpDir, tmpDir, tmpDir)
	if err != nil {
		t.Fatalf("failed to create supervisor: %v", err)
	}
	coll := metrics.NewCollector(tmpDir)

	server := NewServer(socketPath, cfg, sup, coll)

	if err := server.Start(); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	defer server.Stop(context.Background())

	// Give server time to start
	time.Sleep(100 * time.Millisecond)

	// Make multiple concurrent connections
	done := make(chan bool, 10)
	for i := 0; i < 10; i++ {
		go func(id int) {
			conn, err := net.Dial("unix", socketPath)
			if err != nil {
				t.Errorf("connection %d failed to dial: %v", id, err)
				done <- false
				return
			}
			defer conn.Close()

			req := map[string]interface{}{
				"jsonrpc": "2.0",
				"id":      "test-" + string(rune(id)),
				"method":  "daemon.status",
			}

			encoder := json.NewEncoder(conn)
			if err := encoder.Encode(req); err != nil {
				t.Errorf("connection %d failed to encode: %v", id, err)
				done <- false
				return
			}

			decoder := json.NewDecoder(conn)
			var resp Response
			if err := decoder.Decode(&resp); err != nil {
				t.Errorf("connection %d failed to decode: %v", id, err)
				done <- false
				return
			}

			if resp.JSONRPC != "2.0" {
				t.Errorf("connection %d: expected jsonrpc 2.0, got %s", id, resp.JSONRPC)
				done <- false
				return
			}

			done <- true
		}(i)
	}

	// Wait for all connections
	successCount := 0
	for i := 0; i < 10; i++ {
		if <-done {
			successCount++
		}
	}

	if successCount != 10 {
		t.Errorf("expected 10 successful connections, got %d", successCount)
	}
}

func TestInvalidJSONRPCVersion(t *testing.T) {
	tmpDir := t.TempDir()
	socketPath := filepath.Join(tmpDir, "test.sock")

	cfg := &config.Config{}
	sup, err := supervisor.New(cfg, tmpDir, tmpDir, tmpDir)
	if err != nil {
		t.Fatalf("failed to create supervisor: %v", err)
	}
	coll := metrics.NewCollector(tmpDir)

	server := NewServer(socketPath, cfg, sup, coll)

	if err := server.Start(); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	defer server.Stop(context.Background())

	time.Sleep(100 * time.Millisecond)

	conn, err := net.Dial("unix", socketPath)
	if err != nil {
		t.Fatalf("failed to dial: %v", err)
	}
	defer conn.Close()

	// Send invalid jsonrpc version
	req := map[string]interface{}{
		"jsonrpc": "1.0",
		"id":      "test-1",
		"method":  "daemon.status",
	}

	encoder := json.NewEncoder(conn)
	if err := encoder.Encode(req); err != nil {
		t.Fatalf("failed to encode: %v", err)
	}

	decoder := json.NewDecoder(conn)
	var resp Response
	if err := decoder.Decode(&resp); err != nil {
		t.Fatalf("failed to decode: %v", err)
	}

	if resp.Error == nil {
		t.Error("expected error for invalid jsonrpc version")
	}

	if resp.Error.Code != InvalidRequest {
		t.Errorf("expected error code %d, got %d", InvalidRequest, resp.Error.Code)
	}
}

func TestMissingMethod(t *testing.T) {
	tmpDir := t.TempDir()
	socketPath := filepath.Join(tmpDir, "test.sock")

	cfg := &config.Config{}
	sup, err := supervisor.New(cfg, tmpDir, tmpDir, tmpDir)
	if err != nil {
		t.Fatalf("failed to create supervisor: %v", err)
	}
	coll := metrics.NewCollector(tmpDir)

	server := NewServer(socketPath, cfg, sup, coll)

	if err := server.Start(); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	defer server.Stop(context.Background())

	time.Sleep(100 * time.Millisecond)

	conn, err := net.Dial("unix", socketPath)
	if err != nil {
		t.Fatalf("failed to dial: %v", err)
	}
	defer conn.Close()

	// Send request without method
	req := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      "test-1",
	}

	encoder := json.NewEncoder(conn)
	if err := encoder.Encode(req); err != nil {
		t.Fatalf("failed to encode: %v", err)
	}

	decoder := json.NewDecoder(conn)
	var resp Response
	if err := decoder.Decode(&resp); err != nil {
		t.Fatalf("failed to decode: %v", err)
	}

	if resp.Error == nil {
		t.Error("expected error for missing method")
	}

	if resp.Error.Code != InvalidRequest {
		t.Errorf("expected error code %d, got %d", InvalidRequest, resp.Error.Code)
	}
}

func TestMethodNotFound(t *testing.T) {
	tmpDir := t.TempDir()
	socketPath := filepath.Join(tmpDir, "test.sock")

	cfg := &config.Config{}
	sup, err := supervisor.New(cfg, tmpDir, tmpDir, tmpDir)
	if err != nil {
		t.Fatalf("failed to create supervisor: %v", err)
	}
	coll := metrics.NewCollector(tmpDir)

	server := NewServer(socketPath, cfg, sup, coll)

	if err := server.Start(); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	defer server.Stop(context.Background())

	time.Sleep(100 * time.Millisecond)

	conn, err := net.Dial("unix", socketPath)
	if err != nil {
		t.Fatalf("failed to dial: %v", err)
	}
	defer conn.Close()

	// Send request for non-existent method
	req := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      "test-1",
		"method":  "nonexistent.method",
	}

	encoder := json.NewEncoder(conn)
	if err := encoder.Encode(req); err != nil {
		t.Fatalf("failed to encode: %v", err)
	}

	decoder := json.NewDecoder(conn)
	var resp Response
	if err := decoder.Decode(&resp); err != nil {
		t.Fatalf("failed to decode: %v", err)
	}

	if resp.Error == nil {
		t.Error("expected error for non-existent method")
	}

	if resp.Error.Code != MethodNotFound {
		t.Errorf("expected error code %d, got %d", MethodNotFound, resp.Error.Code)
	}
}

func TestParseError(t *testing.T) {
	tmpDir := t.TempDir()
	socketPath := filepath.Join(tmpDir, "test.sock")

	cfg := &config.Config{}
	sup, err := supervisor.New(cfg, tmpDir, tmpDir, tmpDir)
	if err != nil {
		t.Fatalf("failed to create supervisor: %v", err)
	}
	coll := metrics.NewCollector(tmpDir)

	server := NewServer(socketPath, cfg, sup, coll)

	if err := server.Start(); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	defer server.Stop(context.Background())

	time.Sleep(100 * time.Millisecond)

	conn, err := net.Dial("unix", socketPath)
	if err != nil {
		t.Fatalf("failed to dial: %v", err)
	}
	defer conn.Close()

	// Send invalid JSON
	_, err = conn.Write([]byte("{invalid json}\n"))
	if err != nil {
		t.Fatalf("failed to write: %v", err)
	}

	decoder := json.NewDecoder(conn)
	var resp Response
	if err := decoder.Decode(&resp); err != nil {
		t.Fatalf("failed to decode: %v", err)
	}

	if resp.Error == nil {
		t.Error("expected error for invalid JSON")
	}

	if resp.Error.Code != ParseError {
		t.Errorf("expected error code %d, got %d", ParseError, resp.Error.Code)
	}
}

func TestDaemonStatusMethod(t *testing.T) {
	tmpDir := t.TempDir()
	socketPath := filepath.Join(tmpDir, "test.sock")

	cfg := &config.Config{}
	sup, err := supervisor.New(cfg, tmpDir, tmpDir, tmpDir)
	if err != nil {
		t.Fatalf("failed to create supervisor: %v", err)
	}
	coll := metrics.NewCollector(tmpDir)

	server := NewServer(socketPath, cfg, sup, coll)

	if err := server.Start(); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	defer server.Stop(context.Background())

	time.Sleep(100 * time.Millisecond)

	conn, err := net.Dial("unix", socketPath)
	if err != nil {
		t.Fatalf("failed to dial: %v", err)
	}
	defer conn.Close()

	req := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      "test-1",
		"method":  "daemon.status",
	}

	encoder := json.NewEncoder(conn)
	if err := encoder.Encode(req); err != nil {
		t.Fatalf("failed to encode: %v", err)
	}

	decoder := json.NewDecoder(conn)
	var resp Response
	if err := decoder.Decode(&resp); err != nil {
		t.Fatalf("failed to decode: %v", err)
	}

	if resp.Error != nil {
		t.Errorf("unexpected error: %v", resp.Error)
	}

	result, ok := resp.Result.(map[string]interface{})
	if !ok {
		t.Fatal("result should be a map")
	}

	if _, ok := result["uptime_s"]; !ok {
		t.Error("result should contain uptime_s")
	}

	if _, ok := result["pid"]; !ok {
		t.Error("result should contain pid")
	}

	if _, ok := result["version"]; !ok {
		t.Error("result should contain version")
	}
}

func TestMetricsSnapshotMethod(t *testing.T) {
	tmpDir := t.TempDir()
	socketPath := filepath.Join(tmpDir, "test.sock")

	cfg := &config.Config{}
	sup, err := supervisor.New(cfg, tmpDir, tmpDir, tmpDir)
	if err != nil {
		t.Fatalf("failed to create supervisor: %v", err)
	}
	coll := metrics.NewCollector(tmpDir)

	server := NewServer(socketPath, cfg, sup, coll)

	if err := server.Start(); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	defer server.Stop(context.Background())

	time.Sleep(100 * time.Millisecond)

	conn, err := net.Dial("unix", socketPath)
	if err != nil {
		t.Fatalf("failed to dial: %v", err)
	}
	defer conn.Close()

	req := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      "test-1",
		"method":  "metrics.snapshot",
	}

	encoder := json.NewEncoder(conn)
	if err := encoder.Encode(req); err != nil {
		t.Fatalf("failed to encode: %v", err)
	}

	decoder := json.NewDecoder(conn)
	var resp Response
	if err := decoder.Decode(&resp); err != nil {
		t.Fatalf("failed to decode: %v", err)
	}

	if resp.Error != nil {
		t.Errorf("unexpected error: %v", resp.Error)
	}

	result, ok := resp.Result.(map[string]interface{})
	if !ok {
		t.Fatal("result should be a map")
	}

	if _, ok := result["cpu_pct"]; !ok {
		t.Error("result should contain cpu_pct")
	}

	if _, ok := result["ram_mb"]; !ok {
		t.Error("result should contain ram_mb")
	}

	if _, ok := result["disk_mb"]; !ok {
		t.Error("result should contain disk_mb")
	}
}

func TestMetricsServicesMethod(t *testing.T) {
	tmpDir := t.TempDir()
	socketPath := filepath.Join(tmpDir, "test.sock")

	cfg := &config.Config{}
	sup, err := supervisor.New(cfg, tmpDir, tmpDir, tmpDir)
	if err != nil {
		t.Fatalf("failed to create supervisor: %v", err)
	}
	coll := metrics.NewCollector(tmpDir)

	server := NewServer(socketPath, cfg, sup, coll)

	if err := server.Start(); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	defer server.Stop(context.Background())

	time.Sleep(100 * time.Millisecond)

	conn, err := net.Dial("unix", socketPath)
	if err != nil {
		t.Fatalf("failed to dial: %v", err)
	}
	defer conn.Close()

	req := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      "test-1",
		"method":  "metrics.services",
	}

	encoder := json.NewEncoder(conn)
	if err := encoder.Encode(req); err != nil {
		t.Fatalf("failed to encode: %v", err)
	}

	decoder := json.NewDecoder(conn)
	var resp Response
	if err := decoder.Decode(&resp); err != nil {
		t.Fatalf("failed to decode: %v", err)
	}

	if resp.Error != nil {
		t.Errorf("unexpected error: %v", resp.Error)
	}

	// Result can be []interface{} or []map[string]interface{}
	resultBytes, _ := json.Marshal(resp.Result)
	var resultArray []interface{}
	if err := json.Unmarshal(resultBytes, &resultArray); err != nil {
		t.Fatalf("result should be an array: %v", err)
	}
	
	// Array can be empty if no services are running
	_ = resultArray
}

func TestResponseStructure(t *testing.T) {
	resp := Response{
		JSONRPC: "2.0",
		ID:      "test-1",
		Result:  map[string]interface{}{"ok": true},
	}

	if resp.JSONRPC != "2.0" {
		t.Error("JSONRPC should be 2.0")
	}

	if resp.ID != "test-1" {
		t.Errorf("expected ID test-1, got %s", resp.ID)
	}
}

func TestErrorStructure(t *testing.T) {
	err := Error{
		Code:    InternalError,
		Message: "test error",
		Data:    map[string]interface{}{"detail": "more info"},
	}

	if err.Code != InternalError {
		t.Errorf("expected code %d, got %d", InternalError, err.Code)
	}

	if err.Message != "test error" {
		t.Errorf("expected message 'test error', got %s", err.Message)
	}
}
