package storeedgemanager

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

type Supervisor struct {
	mu         sync.Mutex
	workerPath string
	workerURL  string
	dataDir    string
	logDir     string
	cmd        *exec.Cmd
	done       chan error
	token      string
	desired    bool
	closed     bool
	startedAt  *time.Time
	lastExitAt *time.Time
	lastErr    string
}

func NewSupervisor(workerPath, workerURL, dataDir, logDir string) *Supervisor {
	return &Supervisor{workerPath: workerPath, workerURL: strings.TrimRight(workerURL, "/"), dataDir: dataDir, logDir: logDir}
}

func (s *Supervisor) WorkerPath() string { return s.workerPath }

func (s *Supervisor) Start() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return errors.New("manager is shutting down")
	}
	s.desired = true
	if s.cmd != nil && s.cmd.Process != nil && s.cmd.ProcessState == nil {
		return nil
	}
	return s.startLocked()
}

func (s *Supervisor) startLocked() error {
	if _, err := os.Stat(s.workerPath); err != nil {
		return fmt.Errorf("Store Edge worker not found: %w", err)
	}
	tokenRaw := make([]byte, 32)
	if _, err := rand.Read(tokenRaw); err != nil {
		return err
	}
	token := base64.RawURLEncoding.EncodeToString(tokenRaw)
	if err := os.MkdirAll(s.logDir, 0o755); err != nil {
		return err
	}
	logFile, err := os.OpenFile(filepath.Join(s.logDir, "worker.log"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	cmd := exec.Command(s.workerPath)
	cmd.Env = append(os.Environ(),
		"AUTOPARTS_EDGE_DATA_DIR="+s.dataDir,
		"AUTOPARTS_EDGE_MANAGER_TOKEN="+token,
	)
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	if err := cmd.Start(); err != nil {
		_ = logFile.Close()
		return err
	}
	now := time.Now().UTC()
	done := make(chan error, 1)
	s.cmd = cmd
	s.done = done
	s.token = token
	s.startedAt = &now
	s.lastErr = ""
	go func(localCmd *exec.Cmd, localDone chan error, lf *os.File) {
		err := localCmd.Wait()
		_ = lf.Close()
		localDone <- err
		close(localDone)
		s.mu.Lock()
		if s.cmd == localCmd {
			s.cmd = nil
			s.done = nil
			s.token = ""
			n := time.Now().UTC()
			s.lastExitAt = &n
			if err != nil {
				s.lastErr = err.Error()
			} else {
				s.lastErr = ""
			}
			restart := s.desired && !s.closed
			s.mu.Unlock()
			if restart {
				time.Sleep(2 * time.Second)
				_ = s.Start()
			}
			return
		}
		s.mu.Unlock()
	}(cmd, done, logFile)
	return nil
}

func (s *Supervisor) Stop(ctx context.Context) error {
	s.mu.Lock()
	s.desired = false
	cmd, done, token := s.cmd, s.done, s.token
	s.mu.Unlock()
	if cmd == nil || cmd.Process == nil {
		return nil
	}

	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, s.workerURL+"/v1/admin/shutdown", strings.NewReader("{}"))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-AutoParts-Manager-Token", token)
	client := &http.Client{Timeout: 4 * time.Second}
	resp, err := client.Do(req)
	if err == nil {
		_, _ = io.Copy(io.Discard, resp.Body)
		_ = resp.Body.Close()
	}

	if done != nil {
		select {
		case <-done:
			return nil
		case <-ctx.Done():
			_ = cmd.Process.Kill()
			return ctx.Err()
		case <-time.After(7 * time.Second):
			_ = cmd.Process.Kill()
			select {
			case <-done:
			case <-time.After(2 * time.Second):
			}
		}
	}
	return nil
}

func (s *Supervisor) Restart(ctx context.Context) error {
	if err := s.Stop(ctx); err != nil && !errors.Is(err, context.Canceled) && !errors.Is(err, context.DeadlineExceeded) {
		return err
	}
	return s.Start()
}

func (s *Supervisor) Close(ctx context.Context) error {
	s.mu.Lock()
	s.closed = true
	s.desired = false
	s.mu.Unlock()
	return s.Stop(ctx)
}

func (s *Supervisor) Status(ctx context.Context) WorkerStatus {
	s.mu.Lock()
	st := WorkerStatus{State: "stopped", StartedAt: cloneTime(s.startedAt), LastExitAt: cloneTime(s.lastExitAt), LastExitError: s.lastErr}
	if s.cmd != nil && s.cmd.Process != nil && s.cmd.ProcessState == nil {
		st.State = "starting"
		st.PID = s.cmd.Process.Pid
	}
	s.mu.Unlock()

	if st.PID == 0 {
		return st
	}
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, s.workerURL+"/healthz", nil)
	resp, err := (&http.Client{Timeout: 900 * time.Millisecond}).Do(req)
	if err != nil {
		return st
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return st
	}
	var body struct {
		Status  string `json:"status"`
		Version string `json:"version"`
	}
	if json.NewDecoder(resp.Body).Decode(&body) == nil && body.Status == "ok" {
		st.State = "running"
		st.Healthy = true
		st.Version = body.Version
	}
	return st
}

func cloneTime(v *time.Time) *time.Time {
	if v == nil {
		return nil
	}
	x := *v
	return &x
}
