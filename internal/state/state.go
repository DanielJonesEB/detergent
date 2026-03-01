package state

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
)

const (
	stateDir    = ".line"
	pidFile     = "run.pid"
	stationsDir = "stations"
)

// ensureDir creates the .line directory if it doesn't exist.
func ensureDir(repoDir string) error {
	dir := filepath.Join(repoDir, stateDir)
	return os.MkdirAll(dir, 0o755)
}

// ensureStationsDir creates the .line/stations directory and returns its path.
func ensureStationsDir(repoDir string) (string, error) {
	dir := filepath.Join(repoDir, stateDir, stationsDir)
	return dir, os.MkdirAll(dir, 0o755)
}

// removeFile removes a file, returning nil if it doesn't exist.
func removeFile(path string) error {
	err := os.Remove(path)
	if os.IsNotExist(err) {
		return nil
	}
	return err
}

// WritePID writes the runner PID file.
func WritePID(repoDir string, pid int) error {
	if err := ensureDir(repoDir); err != nil {
		return err
	}
	path := filepath.Join(repoDir, stateDir, pidFile)
	return os.WriteFile(path, []byte(strconv.Itoa(pid)), 0o644)
}

// ReadPID reads the runner PID. Returns 0 if no PID file exists.
func ReadPID(repoDir string) (int, error) {
	path := filepath.Join(repoDir, stateDir, pidFile)
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	return strconv.Atoi(string(data))
}

// RemovePID removes the PID file.
func RemovePID(repoDir string) error {
	return removeFile(filepath.Join(repoDir, stateDir, pidFile))
}

// IsProcessRunning checks if a process with the given PID is running.
func IsProcessRunning(pid int) bool {
	if pid <= 0 {
		return false
	}
	process, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	err = process.Signal(syscall.Signal(0))
	return err == nil
}

// KillProcessGroup sends SIGTERM to the process group of the given PID.
// Falls back to killing the process directly if it is not a process group
// leader (e.g. when started from a git hook).
func KillProcessGroup(pid int) error {
	if pid <= 0 {
		return nil
	}
	// Try process group kill first (works if pid is a PGID)
	if err := syscall.Kill(-pid, syscall.SIGTERM); err != nil {
		// Process may not be a group leader (e.g. started from a hook
		// via "line run &"). Fall back to signaling the process directly.
		return syscall.Kill(pid, syscall.SIGTERM)
	}
	return nil
}

// WriteStationPID writes a station's agent PID and start time.
// Format: "PID TIMESTAMP" (e.g., "12345 2024-01-15T10:30:00Z")
func WriteStationPID(repoDir, stationName string, pid int, startTime time.Time) error {
	dir, err := ensureStationsDir(repoDir)
	if err != nil {
		return err
	}
	content := fmt.Sprintf("%d %s", pid, startTime.Format(time.RFC3339))
	return os.WriteFile(filepath.Join(dir, stationName+".pid"), []byte(content), 0o644)
}

// ReadStationPID reads a station's agent PID and start time.
// Returns pid=0 if no PID file exists.
func ReadStationPID(repoDir, stationName string) (int, time.Time, error) {
	path := filepath.Join(repoDir, stateDir, stationsDir, stationName+".pid")
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return 0, time.Time{}, nil
	}
	if err != nil {
		return 0, time.Time{}, err
	}
	parts := strings.SplitN(strings.TrimSpace(string(data)), " ", 2)
	pid, err := strconv.Atoi(parts[0])
	if err != nil {
		return 0, time.Time{}, fmt.Errorf("parsing station PID: %w", err)
	}
	var startTime time.Time
	if len(parts) > 1 {
		startTime, _ = time.Parse(time.RFC3339, parts[1])
	}
	return pid, startTime, nil
}

// RemoveStationPID removes a station's PID file.
func RemoveStationPID(repoDir, stationName string) error {
	return removeFile(filepath.Join(repoDir, stateDir, stationsDir, stationName+".pid"))
}

// KillAllStationAgents kills all running station agent processes and removes
// their PID files. Agents run in their own process groups (Setpgid), so each
// must be killed individually via its process group.
func KillAllStationAgents(repoDir string) {
	dir := filepath.Join(repoDir, stateDir, stationsDir)
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	for _, e := range entries {
		if !strings.HasSuffix(e.Name(), ".pid") {
			continue
		}
		name := strings.TrimSuffix(e.Name(), ".pid")
		pid, _, _ := ReadStationPID(repoDir, name)
		if pid > 0 && IsProcessRunning(pid) {
			_ = KillProcessGroup(pid)
		}
		_ = RemoveStationPID(repoDir, name)
	}
}

// WriteStationFailed writes a marker indicating a station's agent failed.
func WriteStationFailed(repoDir, stationName string) error {
	dir, err := ensureStationsDir(repoDir)
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, stationName+".failed"), []byte("1"), 0o644)
}

// ReadStationFailed returns true if a station has a failure marker.
func ReadStationFailed(repoDir, stationName string) bool {
	path := filepath.Join(repoDir, stateDir, stationsDir, stationName+".failed")
	_, err := os.Stat(path)
	return err == nil
}

// RemoveStationFailed removes a station's failure marker.
func RemoveStationFailed(repoDir, stationName string) error {
	return removeFile(filepath.Join(repoDir, stateDir, stationsDir, stationName+".failed"))
}
