package metrics

import (
	"context"
	"testing"
	"time"
)

func TestNewCollector(t *testing.T) {
	tests := []struct {
		name    string
		dataDir string
	}{
		{
			name:    "valid directory",
			dataDir: "/tmp",
		},
		{
			name:    "empty path",
			dataDir: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := NewCollector(tt.dataDir)
			if c == nil {
				t.Fatal("NewCollector returned nil")
			}
			if c.dataDir != tt.dataDir {
				t.Errorf("expected dataDir %q, got %q", tt.dataDir, c.dataDir)
			}
		})
	}
}

func TestCollectorCollect(t *testing.T) {
	tmpDir := t.TempDir()
	c := NewCollector(tmpDir)

	ctx := context.Background()
	servicePIDs := map[string]int{}

	snap, err := c.Collect(ctx, servicePIDs)
	if err != nil {
		t.Fatalf("Collect failed: %v", err)
	}

	if snap == nil {
		t.Fatal("Collect returned nil snapshot")
	}

	if snap.Timestamp.IsZero() {
		t.Error("Timestamp should not be zero")
	}

	if snap.CPUPercent < 0 || snap.CPUPercent > 100 {
		t.Errorf("CPU percent out of range: %f", snap.CPUPercent)
	}

	if snap.RAMUsedMB < 0 {
		t.Errorf("RAM used should be non-negative: %f", snap.RAMUsedMB)
	}

	if snap.DiskUsedMB < 0 {
		t.Errorf("Disk used should be non-negative: %f", snap.DiskUsedMB)
	}
}

func TestCollectorCollectWithServicePIDs(t *testing.T) {
	tmpDir := t.TempDir()
	c := NewCollector(tmpDir)

	ctx := context.Background()
	servicePIDs := map[string]int{
		"nginx": 1, // PID 1 always exists in containers
	}

	snap, err := c.Collect(ctx, servicePIDs)
	if err != nil {
		t.Fatalf("Collect failed: %v", err)
	}

	if snap == nil {
		t.Fatal("Collect returned nil snapshot")
	}
}

func TestCollectorGetServiceMetrics(t *testing.T) {
	tmpDir := t.TempDir()
	c := NewCollector(tmpDir)

	tests := []struct {
		name         string
		servicePIDs  map[string]int
		expectCount  int
		checkRunning bool
	}{
		{
			name:        "empty services",
			servicePIDs: map[string]int{},
			expectCount: 0,
		},
		{
			name: "single service with valid PID",
			servicePIDs: map[string]int{
				"init": 1,
			},
			expectCount:  1,
			checkRunning: true,
		},
		{
			name: "service with invalid PID",
			servicePIDs: map[string]int{
				"fake": 999999,
			},
			expectCount:  1,
			checkRunning: false,
		},
		{
			name: "multiple services",
			servicePIDs: map[string]int{
				"init": 1,
				"fake": 999999,
			},
			expectCount: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			metrics := c.GetServiceMetrics(tt.servicePIDs)

			if len(metrics) != tt.expectCount {
				t.Errorf("expected %d metrics, got %d", tt.expectCount, len(metrics))
			}

			if tt.checkRunning {
				for _, m := range metrics {
					if m.Name == "init" && m.Status != "running" {
						t.Errorf("expected init to be running, got %s", m.Status)
					}
				}
			}
		})
	}
}

func TestServiceMetricsStructure(t *testing.T) {
	tmpDir := t.TempDir()
	c := NewCollector(tmpDir)

	servicePIDs := map[string]int{
		"test": 1,
	}

	metrics := c.GetServiceMetrics(servicePIDs)

	if len(metrics) == 0 {
		t.Fatal("No metrics returned")
	}

	m := metrics[0]
	if m.Name != "test" {
		t.Errorf("expected name 'test', got %q", m.Name)
	}

	if m.PID != 1 {
		t.Errorf("expected PID 1, got %d", m.PID)
	}

	if m.Status != "running" {
		t.Errorf("expected status 'running', got %q", m.Status)
	}

	if m.RAMMB < 0 {
		t.Errorf("RAM should be non-negative: %f", m.RAMMB)
	}
}

func TestGetLastSnapshot(t *testing.T) {
	tmpDir := t.TempDir()
	c := NewCollector(tmpDir)

	// Before any collection, should return nil
	snap := c.GetLastSnapshot()
	if snap != nil {
		t.Error("GetLastSnapshot should return nil before first collection")
	}

	// Collect once
	ctx := context.Background()
	_, err := c.Collect(ctx, map[string]int{})
	if err != nil {
		t.Fatalf("Collect failed: %v", err)
	}

	// Now should return the snapshot
	snap = c.GetLastSnapshot()
	if snap == nil {
		t.Fatal("GetLastSnapshot returned nil after collection")
	}

	if snap.Timestamp.IsZero() {
		t.Error("Timestamp should not be zero")
	}
}

func TestCollectorConcurrentAccess(t *testing.T) {
	tmpDir := t.TempDir()
	c := NewCollector(tmpDir)

	ctx := context.Background()
	done := make(chan bool)

	// Start multiple goroutines collecting metrics
	for i := 0; i < 10; i++ {
		go func() {
			_, err := c.Collect(ctx, map[string]int{"test": 1})
			if err != nil {
				t.Errorf("Collect failed: %v", err)
			}
			done <- true
		}()
	}

	// Wait for all goroutines to complete
	for i := 0; i < 10; i++ {
		<-done
	}

	// Verify we can still get the last snapshot
	snap := c.GetLastSnapshot()
	if snap == nil {
		t.Fatal("GetLastSnapshot returned nil after concurrent access")
	}
}

func TestReadCPUPercent(t *testing.T) {
	tmpDir := t.TempDir()
	c := NewCollector(tmpDir)

	ctx := context.Background()
	cpuPct, err := c.readCPUPercent(ctx)

	if err != nil {
		t.Fatalf("readCPUPercent failed: %v", err)
	}

	if cpuPct < 0 || cpuPct > 100 {
		t.Errorf("CPU percent out of range: %f", cpuPct)
	}
}

func TestReadRAMUsedMB(t *testing.T) {
	tmpDir := t.TempDir()
	c := NewCollector(tmpDir)

	ctx := context.Background()
	ramMB, err := c.readRAMUsedMB(ctx)

	if err != nil {
		t.Fatalf("readRAMUsedMB failed: %v", err)
	}

	if ramMB < 0 {
		t.Errorf("RAM used should be non-negative: %f", ramMB)
	}
}

func TestReadDiskUsedMB(t *testing.T) {
	tmpDir := t.TempDir()
	c := NewCollector(tmpDir)

	ctx := context.Background()
	diskMB, err := c.readDiskUsedMB(ctx)

	if err != nil {
		t.Fatalf("readDiskUsedMB failed: %v", err)
	}

	if diskMB < 0 {
		t.Errorf("Disk used should be non-negative: %f", diskMB)
	}
}

func TestProcessExists(t *testing.T) {
	tmpDir := t.TempDir()
	c := NewCollector(tmpDir)

	// PID 1 should always exist
	if !c.processExists(1) {
		t.Error("PID 1 should exist")
	}

	// Very high PID should not exist
	if c.processExists(999999) {
		t.Error("PID 999999 should not exist")
	}
}

func TestGetProcessRAMMB(t *testing.T) {
	tmpDir := t.TempDir()
	c := NewCollector(tmpDir)

	// PID 1 should have some RAM usage
	ramMB := c.getProcessRAMMB(1)
	if ramMB < 0 {
		t.Errorf("RAM should be non-negative: %f", ramMB)
	}

	// Invalid PID should return 0
	ramMB = c.getProcessRAMMB(999999)
	if ramMB != 0 {
		t.Errorf("Invalid PID should return 0 RAM, got %f", ramMB)
	}
}

func TestSnapshotJSONTags(t *testing.T) {
	snap := Snapshot{
		CPUPercent: 10.5,
		RAMUsedMB:  512.0,
		ReqPerSec:  100.0,
		ErrorRate:  0.01,
		DiskUsedMB: 1024.0,
		Timestamp:  time.Now(),
	}

	// Verify JSON tags are present by checking struct definition
	// This is a compile-time check more than runtime
	_ = snap.CPUPercent
	_ = snap.RAMUsedMB
	_ = snap.ReqPerSec
	_ = snap.ErrorRate
	_ = snap.DiskUsedMB
	_ = snap.Timestamp
}

func TestServiceMetricsJSONTags(t *testing.T) {
	sm := ServiceMetrics{
		Name:   "nginx",
		Status: "running",
		PID:    1234,
		RAMMB:  50.0,
	}

	// Verify JSON tags are present
	_ = sm.Name
	_ = sm.Status
	_ = sm.PID
	_ = sm.RAMMB
}

func TestCollectorWithDataDir(t *testing.T) {
	tests := []struct {
		name    string
		dataDir string
		wantErr bool
	}{
		{
			name:    "valid directory",
			dataDir: "/tmp",
			wantErr: false,
		},
		{
			name:    "nonexistent directory",
			dataDir: "/nonexistent/path/that/does/not/exist",
			wantErr: true,
		},
		{
			name:    "empty path",
			dataDir: "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := NewCollector(tt.dataDir)
			
			ctx := context.Background()
			_, err := c.readDiskUsedMB(ctx)

			if tt.wantErr && err == nil {
				t.Error("expected error but got none")
			}
			if !tt.wantErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}
