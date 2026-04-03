// Package metrics provides system and service metrics collection.
package metrics

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
)

// Snapshot contains a point-in-time view of system metrics.
type Snapshot struct {
	CPUPercent float64   `json:"cpu_pct"`
	RAMUsedMB  float64   `json:"ram_mb"`
	ReqPerSec  float64   `json:"req_per_s"`
	ErrorRate  float64   `json:"error_rate"`
	DiskUsedMB float64   `json:"disk_mb"`
	Timestamp  time.Time `json:"timestamp"`
}

// ServiceMetrics contains metrics for a single managed service.
type ServiceMetrics struct {
	Name   string  `json:"name"`
	Status string  `json:"status"` // "running", "stopped", "starting"
	PID    int     `json:"pid"`
	RAMMB  float64 `json:"ram_mb"`
}

// Collector gathers system and service metrics.
type Collector struct {
	mu           sync.RWMutex
	lastSnapshot *Snapshot
	dataDir      string
}

// NewCollector creates a new metrics collector.
func NewCollector(dataDir string) *Collector {
	return &Collector{
		dataDir: dataDir,
	}
}

// Collect gathers a new metrics snapshot.
func (c *Collector) Collect(ctx context.Context, servicePIDs map[string]int) (*Snapshot, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	snap := &Snapshot{
		Timestamp: time.Now(),
	}

	// Collect CPU usage
	cpuPct, err := c.readCPUPercent(ctx)
	if err != nil {
		cpuPct = 0
	}
	snap.CPUPercent = cpuPct

	// Collect RAM usage
	ramMB, err := c.readRAMUsedMB(ctx)
	if err != nil {
		ramMB = 0
	}
	snap.RAMUsedMB = ramMB

	// Collect disk usage
	diskMB, err := c.readDiskUsedMB(ctx)
	if err != nil {
		diskMB = 0
	}
	snap.DiskUsedMB = diskMB

	// Request rate and error rate would come from nginx logs in a real implementation
	// For now, we return 0 as these require log parsing
	snap.ReqPerSec = 0
	snap.ErrorRate = 0

	c.lastSnapshot = snap
	return snap, nil
}

// readCPUPercent reads CPU usage from /proc/stat.
func (c *Collector) readCPUPercent(ctx context.Context) (float64, error) {
	f, err := os.Open("/proc/stat")
	if err != nil {
		return 0, fmt.Errorf("open /proc/stat: %w", err)
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	if !scanner.Scan() {
		return 0, fmt.Errorf("empty /proc/stat")
	}

	line := scanner.Text()
	fields := strings.Fields(line)
	if len(fields) < 5 || fields[0] != "cpu" {
		return 0, fmt.Errorf("invalid /proc/stat format")
	}

	// Parse CPU times (user, nice, system, idle)
	user, _ := strconv.ParseFloat(fields[1], 64)
	nice, _ := strconv.ParseFloat(fields[2], 64)
	system, _ := strconv.ParseFloat(fields[3], 64)
	idle, _ := strconv.ParseFloat(fields[4], 64)

	total := user + nice + system + idle
	if total == 0 {
		return 0, nil
	}

	used := user + nice + system
	return (used / total) * 100, nil
}

// readRAMUsedMB reads RAM usage from /proc/meminfo.
func (c *Collector) readRAMUsedMB(ctx context.Context) (float64, error) {
	f, err := os.Open("/proc/meminfo")
	if err != nil {
		return 0, fmt.Errorf("open /proc/meminfo: %w", err)
	}
	defer f.Close()

	var memTotal, memAvailable uint64

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}

		key := strings.TrimSuffix(fields[0], ":")
		value, _ := strconv.ParseUint(fields[1], 10, 64)

		switch key {
		case "MemTotal":
			memTotal = value
		case "MemAvailable":
			memAvailable = value
		}
	}

	if memTotal == 0 {
		return 0, fmt.Errorf("could not read MemTotal")
	}

	// Convert from kB to MB
	usedKB := memTotal - memAvailable
	return float64(usedKB) / 1024, nil
}

// readDiskUsedMB reads disk usage for the data directory.
func (c *Collector) readDiskUsedMB(ctx context.Context) (float64, error) {
	var stat syscall.Statfs_t
	if err := syscall.Statfs(c.dataDir, &stat); err != nil {
		return 0, fmt.Errorf("statfs %s: %w", c.dataDir, err)
	}

	// Calculate used space in MB
	total := uint64(stat.Blocks) * uint64(stat.Bsize)
	available := uint64(stat.Bavail) * uint64(stat.Bsize)
	used := total - available

	return float64(used) / (1024 * 1024), nil
}

// GetServiceMetrics collects metrics for managed services.
func (c *Collector) GetServiceMetrics(servicePIDs map[string]int) []ServiceMetrics {
	var metrics []ServiceMetrics

	for name, pid := range servicePIDs {
		sm := ServiceMetrics{
			Name: name,
			PID:  pid,
		}

		// Check if process exists
		if c.processExists(pid) {
			sm.Status = "running"
			sm.RAMMB = c.getProcessRAMMB(pid)
		} else {
			sm.Status = "stopped"
			sm.PID = 0
			sm.RAMMB = 0
		}

		metrics = append(metrics, sm)
	}

	return metrics
}

// processExists checks if a process with the given PID exists.
func (c *Collector) processExists(pid int) bool {
	_, err := os.Stat(fmt.Sprintf("/proc/%d", pid))
	return err == nil
}

// getProcessRAMMB gets the RSS memory usage of a process in MB.
func (c *Collector) getProcessRAMMB(pid int) float64 {
	path := fmt.Sprintf("/proc/%d/status", pid)
	f, err := os.Open(path)
	if err != nil {
		return 0
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "VmRSS:") {
			fields := strings.Fields(line)
			if len(fields) >= 2 {
				kb, _ := strconv.ParseFloat(fields[1], 64)
				return kb / 1024 // Convert kB to MB
			}
		}
	}

	return 0
}

// GetLastSnapshot returns the most recent metrics snapshot.
func (c *Collector) GetLastSnapshot() *Snapshot {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.lastSnapshot
}
