// Package supervisor provides process supervision for pocketd services.
// It handles spawning, monitoring, and automatic restarting of services
// like Nginx, PHP-FPM, MariaDB, Redis, HAProxy, and Node.js inside proot.
package supervisor

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"sync"
	"time"

	"github.com/pocketserver/pocketd/internal/config"
)

// ServiceStatus represents the current state of a managed service.
type ServiceStatus string

const (
	StatusStopped  ServiceStatus = "stopped"
	StatusStarting ServiceStatus = "starting"
	StatusRunning  ServiceStatus = "running"
	StatusStopping ServiceStatus = "stopping"
	StatusFailed   ServiceStatus = "failed"
)

// ServiceInfo contains runtime information about a managed service.
type ServiceInfo struct {
	Name       string        `json:"name"`
	Status     ServiceStatus `json:"status"`
	PID        int           `json:"pid,omitempty"`
	RAMMB      int           `json:"ram_mb,omitempty"`
	CPUPercent float64       `json:"cpu_percent,omitempty"`
	Uptime     time.Duration `json:"uptime_s,omitempty"`
	Error      string        `json:"error,omitempty"`
}

// ProcessManager manages the lifecycle of a single service process.
type ProcessManager struct {
	name         string
	cmd          *exec.Cmd
	status       ServiceStatus
	pid          int
	startTime    time.Time
	restartCount int
	maxRestarts  int
	mu           sync.RWMutex
	error        string
}

// Supervisor manages all server processes inside the proot environment.
type Supervisor struct {
	config      *config.Config
	rootfsPath  string
	projectsDir string
	logDir      string
	services    map[string]*ProcessManager
	ctx         context.Context
	cancel      context.CancelFunc
	wg          sync.WaitGroup
	mu          sync.RWMutex
}

// New creates a new Supervisor instance.
func New(cfg *config.Config, rootfsPath, projectsDir, logDir string) (*Supervisor, error) {
	if cfg == nil {
		return nil, fmt.Errorf("config cannot be nil")
	}
	if rootfsPath == "" {
		return nil, fmt.Errorf("rootfs path cannot be empty")
	}
	if projectsDir == "" {
		return nil, fmt.Errorf("projects directory cannot be empty")
	}
	if logDir == "" {
		return nil, fmt.Errorf("log directory cannot be empty")
	}

	ctx, cancel := context.WithCancel(context.Background())

	return &Supervisor{
		config:      cfg,
		rootfsPath:  rootfsPath,
		projectsDir: projectsDir,
		logDir:      logDir,
		services:    make(map[string]*ProcessManager),
		ctx:         ctx,
		cancel:      cancel,
	}, nil
}

// Start initializes and starts all enabled services based on the configuration.
func (s *Supervisor) Start() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Ensure log directory exists
	if err := os.MkdirAll(s.logDir, 0755); err != nil {
		return fmt.Errorf("failed to create log directory: %w", err)
	}

	// Start Nginx (always required)
	if err := s.startNginx(); err != nil {
		return fmt.Errorf("failed to start nginx: %w", err)
	}

	// Start PHP-FPM if enabled
	if s.config.Stack.PHP {
		if err := s.startPHPFPM(); err != nil {
			return fmt.Errorf("failed to start php-fpm: %w", err)
		}
	}

	// Start MariaDB (always required for now)
	if err := s.startMariaDB(); err != nil {
		return fmt.Errorf("failed to start mariadb: %w", err)
	}

	// Start Redis if enabled
	if s.config.Stack.Redis {
		if err := s.startRedis(); err != nil {
			return fmt.Errorf("failed to start redis: %w", err)
		}
	}

	// Start Node.js if enabled
	if s.config.Stack.NodeJS {
		if err := s.startNodeJS(); err != nil {
			return fmt.Errorf("failed to start nodejs: %w", err)
		}
	}

	return nil
}

// Stop gracefully shuts down all managed services.
func (s *Supervisor) Stop(graceful bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	var lastErr error

	// Stop in reverse order of dependencies
	serviceOrder := []string{"nodejs", "redis", "mariadb", "php-fpm", "nginx"}

	for _, name := range serviceOrder {
		if pm, ok := s.services[name]; ok {
			if err := s.stopService(pm, graceful); err != nil {
				lastErr = fmt.Errorf("failed to stop %s: %w", name, err)
			}
		}
	}

	return lastErr
}

// Restart restarts all services.
func (s *Supervisor) Restart() error {
	if err := s.Stop(true); err != nil {
		return fmt.Errorf("failed to stop services: %w", err)
	}

	time.Sleep(500 * time.Millisecond)

	if err := s.Start(); err != nil {
		return fmt.Errorf("failed to start services: %w", err)
	}

	return nil
}

// GetStatus returns the status of all managed services.
func (s *Supervisor) GetStatus() []ServiceInfo {
	s.mu.RLock()
	defer s.mu.RUnlock()

	infos := make([]ServiceInfo, 0, len(s.services))

	for _, pm := range s.services {
		infos = append(infos, pm.getInfo())
	}

	return infos
}

// GetServiceStatus returns the status of a specific service.
func (s *Supervisor) GetServiceStatus(name string) (*ServiceInfo, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	pm, ok := s.services[name]
	if !ok {
		return nil, fmt.Errorf("service not found: %s", name)
	}

	info := pm.getInfo()
	return &info, nil
}

// startService starts a service with the given command and monitors it.
func (s *Supervisor) startService(name string, cmd *exec.Cmd, maxRestarts int) error {
	pm := &ProcessManager{
		name:        name,
		cmd:         cmd,
		status:      StatusStarting,
		maxRestarts: maxRestarts,
	}

	s.services[name] = pm

	// Start the process
	if err := cmd.Start(); err != nil {
		pm.status = StatusFailed
		pm.mu.Lock()
		pm.pid = -1
		pm.mu.Unlock()
		return fmt.Errorf("failed to start %s: %w", name, err)
	}

	pm.mu.Lock()
	pm.pid = cmd.Process.Pid
	pm.startTime = time.Now()
	pm.mu.Unlock()
	pm.status = StatusRunning

	// Start monitoring goroutine
	s.wg.Add(1)
	go s.monitorService(pm)

	return nil
}

// monitorService watches a service and restarts it on unexpected exit.
func (s *Supervisor) monitorService(pm *ProcessManager) {
	defer s.wg.Done()

	for {
		select {
		case <-s.ctx.Done():
			return
		default:
			// Wait for process to exit
			err := pm.cmd.Wait()

			pm.mu.Lock()
			currentStatus := pm.status
			pm.mu.Unlock()

			// If we're stopping intentionally, don't restart
			if currentStatus == StatusStopping {
				pm.mu.Lock()
				pm.status = StatusStopped
				pm.mu.Unlock()
				return
			}

			// Process exited unexpectedly
			pm.mu.Lock()
			pm.status = StatusFailed
			if err != nil {
				pm.error = err.Error()
			}
			restartCount := pm.restartCount
			pm.mu.Unlock()

			// Check if we should restart
			if restartCount >= pm.maxRestarts {
				// Max restarts reached, give up
				return
			}

			// Calculate backoff delay: 1s, 2s, 4s, 8s, max 30s
			delay := time.Duration(1<<uint(restartCount)) * time.Second
			if delay > 30*time.Second {
				delay = 30 * time.Second
			}

			// Wait before restart attempt
			select {
			case <-time.After(delay):
			case <-s.ctx.Done():
				return
			}

			// Attempt restart
			pm.mu.Lock()
			pm.status = StatusStarting
			pm.restartCount++
			pm.mu.Unlock()

			// Recreate and start the command
			if err := s.restartService(pm); err != nil {
				pm.mu.Lock()
				pm.status = StatusFailed
				pm.error = err.Error()
				pm.mu.Unlock()
				continue
			}

			pm.mu.Lock()
			pm.status = StatusRunning
			pm.mu.Unlock()
		}
	}
}

// restartService recreates and starts a service process.
func (s *Supervisor) restartService(pm *ProcessManager) error {
	// This will be implemented differently for each service type
	// For now, we'll just return an error - specific implementations
	// will override this behavior
	return fmt.Errorf("restart not implemented for %s", pm.name)
}

// stopService stops a service process.
func (s *Supervisor) stopService(pm *ProcessManager, graceful bool) error {
	pm.mu.Lock()
	if pm.status == StatusStopped || pm.status == StatusFailed {
		pm.mu.Unlock()
		return nil
	}
	pm.status = StatusStopping
	pid := pm.pid
	pm.mu.Unlock()

	if pid <= 0 {
		return nil
	}

	// Send SIGTERM for graceful shutdown
	sig := os.Interrupt
	if !graceful {
		sig = os.Kill
	}

	if err := pm.cmd.Process.Signal(sig); err != nil {
		// Process may have already exited
		if err.Error() != "os: process already finished" {
			return fmt.Errorf("failed to signal process: %w", err)
		}
		return nil
	}

	// Wait for graceful shutdown timeout
	if graceful {
		done := make(chan error, 1)
		go func() {
			done <- pm.cmd.Wait()
		}()

		select {
		case <-done:
			return nil
		case <-time.After(10 * time.Second):
			// Force kill if graceful shutdown times out
			pm.cmd.Process.Kill()
			return nil
		}
	}

	return nil
}

// Close shuts down the supervisor and all managed services.
func (s *Supervisor) Close() error {
	s.cancel()
	return s.Stop(true)
}

// getInfo returns the current status information for a process.
func (pm *ProcessManager) getInfo() ServiceInfo {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	info := ServiceInfo{
		Name:   pm.name,
		Status: pm.status,
		PID:    pm.pid,
	}

	if pm.startTime.IsZero() {
		info.Uptime = 0
	} else {
		info.Uptime = time.Since(pm.startTime)
	}

	if pm.error != "" {
		info.Error = pm.error
	}

	// RAM and CPU would be read from /proc/{pid}/ in a real implementation
	// This is handled by the metrics package

	return info
}

// buildProotCommand creates an exec.Cmd that runs inside the proot environment.
func (s *Supervisor) buildProotCommand(args ...string) *exec.Cmd {
	// proot command format:
	// proot -r <rootfs> -b <bind_mounts> -- <command> <args>
	
	cmdArgs := []string{
		"-r", s.rootfsPath,
		"-b", s.projectsDir + ":" + s.projectsDir,
		"-w", "/",
	}
	
	cmdArgs = append(cmdArgs, "--")
	cmdArgs = append(cmdArgs, args...)

	cmd := exec.CommandContext(s.ctx, "proot", cmdArgs...)
	
	// Set up environment
	cmd.Env = []string{
		"PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin",
		"HOME=/root",
	}

	return cmd
}
