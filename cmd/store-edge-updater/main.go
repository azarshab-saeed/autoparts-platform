package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"
)

func main() {
	var pid int
	var managerCurrent, managerNew, workerCurrent, workerNew, serviceMode, serviceName, logPath string
	flag.IntVar(&pid, "pid", 0, "manager process id")
	flag.StringVar(&managerCurrent, "manager-current", "", "current manager executable")
	flag.StringVar(&managerNew, "manager-new", "", "staged manager executable")
	flag.StringVar(&workerCurrent, "worker-current", "", "current worker executable")
	flag.StringVar(&workerNew, "worker-new", "", "staged worker executable")
	flag.StringVar(&serviceMode, "service-mode", "none", "windows|systemd-user|none")
	flag.StringVar(&serviceName, "service-name", "", "service name")
	flag.StringVar(&logPath, "log", "", "updater log path")
	flag.Parse()
	if logPath != "" {
		if err := os.MkdirAll(filepath.Dir(logPath), 0o755); err == nil {
			if f, err := os.OpenFile(logPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600); err == nil {
				defer f.Close()
				log.SetOutput(f)
			}
		}
	}
	if managerCurrent == "" || workerCurrent == "" || workerNew == "" {
		log.Fatal("update paths are incomplete")
	}
	if err := runUpdate(pid, managerCurrent, managerNew, workerCurrent, workerNew, serviceMode, serviceName); err != nil {
		log.Fatal(err)
	}
}

func runUpdate(pid int, managerCurrent, managerNew, workerCurrent, workerNew, serviceMode, serviceName string) error {
	log.Printf("starting Store Edge update service_mode=%s pid=%d", serviceMode, pid)
	switch serviceMode {
	case "windows":
		if runtime.GOOS != "windows" {
			return errors.New("windows update mode requires Windows")
		}
		if serviceName == "" {
			return errors.New("Windows service name is empty")
		}
		_ = run("sc.exe", "stop", serviceName)
		if err := waitForReplaceable(managerCurrent, 35*time.Second); err != nil {
			return err
		}
	case "systemd-user":
		if serviceName == "" {
			return errors.New("systemd service name is empty")
		}
		_ = run("systemctl", "--user", "stop", serviceName)
		if err := waitForPIDExit(pid, 20*time.Second); err != nil {
			return err
		}
	case "none":
		return errors.New("self-update is not supported outside an installed service")
	default:
		return fmt.Errorf("unknown service mode %q", serviceMode)
	}

	if err := replace(workerCurrent, workerNew); err != nil {
		return fmt.Errorf("replace worker: %w", err)
	}
	if managerNew != "" {
		if err := replace(managerCurrent, managerNew); err != nil {
			return fmt.Errorf("replace manager: %w", err)
		}
	}

	switch serviceMode {
	case "windows":
		if err := run("sc.exe", "start", serviceName); err != nil {
			return err
		}
	case "systemd-user":
		if err := run("systemctl", "--user", "start", serviceName); err != nil {
			return err
		}
	}
	log.Printf("Store Edge update completed")
	return nil
}

func replace(current, staged string) error {
	if _, err := os.Stat(staged); err != nil {
		return err
	}
	backup := current + ".bak"
	_ = os.Remove(backup)
	var last error
	for i := 0; i < 40; i++ {
		if err := os.Rename(current, backup); err != nil {
			last = err
			time.Sleep(500 * time.Millisecond)
			continue
		}
		if err := copyFile(staged, current); err != nil {
			_ = os.Rename(backup, current)
			return err
		}
		_ = os.Chmod(current, 0o755)
		_ = os.Remove(backup)
		_ = os.Remove(staged)
		return nil
	}
	return fmt.Errorf("current executable stayed locked: %w", last)
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o755)
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(out, in)
	syncErr := out.Sync()
	closeErr := out.Close()
	if copyErr != nil {
		return copyErr
	}
	if syncErr != nil {
		return syncErr
	}
	return closeErr
}

func waitForReplaceable(path string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		probe := path + ".update-probe"
		if err := os.Rename(path, probe); err == nil {
			_ = os.Rename(probe, path)
			return nil
		}
		time.Sleep(500 * time.Millisecond)
	}
	return fmt.Errorf("manager executable did not become replaceable: %s", path)
}

func waitForPIDExit(pid int, timeout time.Duration) error {
	if pid <= 0 {
		return nil
	}
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if !processExists(pid) {
			return nil
		}
		time.Sleep(300 * time.Millisecond)
	}
	return fmt.Errorf("manager pid %d did not exit", pid)
}

func processExists(pid int) bool {
	if runtime.GOOS == "linux" {
		_, err := os.Stat(filepath.Join("/proc", strconv.Itoa(pid)))
		return err == nil
	}
	p, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	return p.Signal(os.Signal(nil)) == nil
}

func run(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s %s: %w: %s", name, strings.Join(args, " "), err, strings.TrimSpace(string(out)))
	}
	return nil
}
