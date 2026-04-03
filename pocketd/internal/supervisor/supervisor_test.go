package supervisor

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/pocketserver/pocketd/internal/config"
)

// TestNewSupervisor tests the supervisor constructor.
func TestNewSupervisor(t *testing.T) {
	tests := []struct {
		name        string
		cfg         *config.Config
		rootfsPath  string
		projectsDir string
		logDir      string
		wantErr     bool
		errContains string
	}{
		{
			name:        "nil config returns error",
			cfg:         nil,
			rootfsPath:  "/tmp/rootfs",
			projectsDir: "/tmp/projects",
			logDir:      "/tmp/logs",
			wantErr:     true,
			errContains: "config cannot be nil",
		},
		{
			name:        "empty rootfs path returns error",
			cfg:         config.Default(),
			rootfsPath:  "",
			projectsDir: "/tmp/projects",
			logDir:      "/tmp/logs",
			wantErr:     true,
			errContains: "rootfs path cannot be empty",
		},
		{
			name:        "empty projects dir returns error",
			cfg:         config.Default(),
			rootfsPath:  "/tmp/rootfs",
			projectsDir: "",
			logDir:      "/tmp/logs",
			wantErr:     true,
			errContains: "projects directory cannot be empty",
		},
		{
			name:        "empty log dir returns error",
			cfg:         config.Default(),
			rootfsPath:  "/tmp/rootfs",
			projectsDir: "/tmp/projects",
			logDir:      "",
			wantErr:     true,
			errContains: "log directory cannot be empty",
		},
		{
			name:        "valid config creates supervisor",
			cfg:         config.Default(),
			rootfsPath:  "/tmp/rootfs",
			projectsDir: "/tmp/projects",
			logDir:      "/tmp/logs",
			wantErr:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s, err := New(tt.cfg, tt.rootfsPath, tt.projectsDir, tt.logDir)

			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error but got nil")
					return
				}
				if tt.errContains != "" && !contains(err.Error(), tt.errContains) {
					t.Errorf("expected error to contain %q, got %q", tt.errContains, err.Error())
				}
				if s != nil {
					t.Errorf("expected nil supervisor on error, got %v", s)
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
					return
				}
				if s == nil {
					t.Errorf("expected supervisor, got nil")
					return
				}
				// Clean up
				s.Close()
			}
		})
	}
}

// TestSupervisorStartStop tests basic start/stop lifecycle.
func TestSupervisorStartStop(t *testing.T) {
	// Create temporary directories
	tmpDir := t.TempDir()
	rootfsPath := filepath.Join(tmpDir, "rootfs")
	projectsDir := filepath.Join(tmpDir, "projects")
	logDir := filepath.Join(tmpDir, "logs")

	if err := os.MkdirAll(rootfsPath, 0755); err != nil {
		t.Fatalf("failed to create rootfs: %v", err)
	}
	if err := os.MkdirAll(projectsDir, 0755); err != nil {
		t.Fatalf("failed to create projects: %v", err)
	}

	cfg := config.Default()
	cfg.Stack.PHP = false
	cfg.Stack.Redis = false
	cfg.Stack.NodeJS = false

	s, err := New(cfg, rootfsPath, projectsDir, logDir)
	if err != nil {
		t.Fatalf("failed to create supervisor: %v", err)
	}
	defer s.Close()

	// Start should fail gracefully when proot is not available
	// (which is expected in test environment)
	err = s.Start()
	
	// We expect an error since proot won't be available in test env
	// The important thing is that it doesn't panic
	if err == nil {
		// If it somehow succeeded, try to stop
		if err := s.Stop(true); err != nil {
			t.Errorf("Stop failed: %v", err)
		}
	}
}

// TestSupervisorGetStatus tests status retrieval.
func TestSupervisorGetStatus(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := config.Default()

	s, err := New(cfg, tmpDir, tmpDir, tmpDir)
	if err != nil {
		t.Fatalf("failed to create supervisor: %v", err)
	}
	defer s.Close()

	// Initially should return empty list
	status := s.GetStatus()
	if len(status) != 0 {
		t.Errorf("expected empty status list, got %d items", len(status))
	}

	// GetServiceStatus for non-existent service should error
	_, err = s.GetServiceStatus("nginx")
	if err == nil {
		t.Errorf("expected error for non-existent service")
	}
}

// TestSupervisorRestart tests the restart functionality.
func TestSupervisorRestart(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := config.Default()
	cfg.Stack.PHP = false
	cfg.Stack.Redis = false
	cfg.Stack.NodeJS = false

	s, err := New(cfg, tmpDir, tmpDir, tmpDir)
	if err != nil {
		t.Fatalf("failed to create supervisor: %v", err)
	}
	defer s.Close()

	// Restart without starting should handle gracefully
	err = s.Restart()
	// May fail due to missing proot, but shouldn't panic
	_ = err
}

// TestProcessManagerGetInfo tests the getInfo method.
func TestProcessManagerGetInfo(t *testing.T) {
	pm := &ProcessManager{
		name:      "test-service",
		status:    StatusRunning,
		pid:       12345,
		startTime: time.Now().Add(-1 * time.Hour),
		error:     "",
	}

	info := pm.getInfo()

	if info.Name != "test-service" {
		t.Errorf("expected name test-service, got %s", info.Name)
	}
	if info.Status != StatusRunning {
		t.Errorf("expected status running, got %s", info.Status)
	}
	if info.PID != 12345 {
		t.Errorf("expected PID 12345, got %d", info.PID)
	}
	if info.Uptime < time.Hour {
		t.Errorf("expected uptime >= 1 hour, got %v", info.Uptime)
	}
	if info.Error != "" {
		t.Errorf("expected empty error, got %s", info.Error)
	}
}

// TestProcessManagerGetInfoWithError tests getInfo with error state.
func TestProcessManagerGetInfoWithError(t *testing.T) {
	pm := &ProcessManager{
		name:      "failed-service",
		status:    StatusFailed,
		pid:       -1,
		startTime: time.Time{},
		error:     "process crashed",
	}

	info := pm.getInfo()

	if info.Status != StatusFailed {
		t.Errorf("expected status failed, got %s", info.Status)
	}
	if info.Error != "process crashed" {
		t.Errorf("expected error 'process crashed', got %s", info.Error)
	}
	if info.Uptime != 0 {
		t.Errorf("expected uptime 0 for zero start time, got %v", info.Uptime)
	}
}

// TestSupervisorContextCancellation tests that context cancellation stops monitoring.
func TestSupervisorContextCancellation(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := config.Default()

	s, err := New(cfg, tmpDir, tmpDir, tmpDir)
	if err != nil {
		t.Fatalf("failed to create supervisor: %v", err)
	}

	// Cancel immediately
	s.cancel()

	// Give goroutines time to clean up
	time.Sleep(100 * time.Millisecond)

	// Should not panic or hang
}

// TestBuildProotCommand tests proot command construction.
func TestBuildProotCommand(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := config.Default()

	s, err := New(cfg, tmpDir, tmpDir, tmpDir)
	if err != nil {
		t.Fatalf("failed to create supervisor: %v", err)
	}
	defer s.Close()

	cmd := s.buildProotCommand("nginx", "-t")

	if cmd == nil {
		t.Fatal("expected non-nil command")
	}

	// Verify command has correct structure
	if len(cmd.Args) < 6 {
		t.Errorf("expected at least 6 args, got %d: %v", len(cmd.Args), cmd.Args)
	}

	// Check for required flags
	foundRoot := false
	foundBind := false
	foundSep := false
	for i, arg := range cmd.Args {
		if arg == "-r" && i+1 < len(cmd.Args) && cmd.Args[i+1] == tmpDir {
			foundRoot = true
		}
		if arg == "-b" && i+1 < len(cmd.Args) {
			foundBind = true
		}
		if arg == "--" {
			foundSep = true
		}
	}

	if !foundRoot {
		t.Error("expected -r flag with rootfs path")
	}
	if !foundBind {
		t.Error("expected -b flag for bind mount")
	}
	if !foundSep {
		t.Error("expected -- separator")
	}
}

// TestServiceStatusConstants tests that status constants are defined.
func TestServiceStatusConstants(t *testing.T) {
	statuses := []ServiceStatus{
		StatusStopped,
		StatusStarting,
		StatusRunning,
		StatusStopping,
		StatusFailed,
	}

	expected := []string{"stopped", "starting", "running", "stopping", "failed"}
	for i, status := range statuses {
		if string(status) != expected[i] {
			t.Errorf("expected status %q, got %q", expected[i], status)
		}
	}
}

// Helper function to check if a string contains a substring.
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) && findSubstring(s, substr))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// MockProcessManager creates a ProcessManager for testing.
func MockProcessManager(name string, status ServiceStatus, pid int) *ProcessManager {
	return &ProcessManager{
		name:   name,
		status: status,
		pid:    pid,
	}
}
