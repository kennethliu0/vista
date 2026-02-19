package main

import (
	"bufio"
	"context"
	"fmt"
	"os/exec"
	"sync"
	"syscall"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

const MaxLogLines = 10000

// Status represents the running state of a service.
type Status int

const (
	Stopped Status = iota
	Running
	Stopping
	Error
)

func (s Status) String() string {
	switch s {
	case Running:
		return "Running"
	case Stopping:
		return "Stopping"
	case Error:
		return "Error"
	default:
		return "Stopped"
	}
}

// Service represents a managed process in the monorepo.
type Service struct {
	Name   string
	Cmd    string
	Dir    string
	Status Status
	Logs   []logEntry
	PID    int

	cmd    *exec.Cmd
	mu     sync.Mutex       // protects cancel and done
	cancel context.CancelFunc
	done   chan struct{} // closed when the scanner goroutine exits
}

// Title implements list.DefaultItem.
func (s *Service) Title() string {
	return fmt.Sprintf("%s %s", statusDot(s.Status), s.Name)
}

// Description implements list.DefaultItem.
func (s *Service) Description() string {
	return s.Status.String()
}

// FilterValue implements list.Item.
func (s *Service) FilterValue() string {
	return s.Name
}

// Start launches the service process and streams logs to logCh.
func (s *Service) Start(logCh chan<- tea.Msg) tea.Cmd {
	if s.Status == Running || s.Status == Stopping {
		return nil
	}
	return func() tea.Msg {

		ctx, cancel := context.WithCancel(context.Background())
		doneCh := make(chan struct{})
		s.mu.Lock()
		s.cancel = cancel
		s.done = doneCh
		s.mu.Unlock()

		s.cmd = exec.CommandContext(ctx, "sh", "-c", s.Cmd)
		s.cmd.Dir = s.Dir
		s.cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

		stdout, err := s.cmd.StdoutPipe()
		if err != nil {
			return serviceStatusMsg{serviceName: s.Name, status: Error, err: err}
		}
		s.cmd.Stderr = s.cmd.Stdout // merge stderr into stdout

		if err := s.cmd.Start(); err != nil {
			return serviceStatusMsg{serviceName: s.Name, status: Error, err: err}
		}

		pid := s.cmd.Process.Pid

		// Stream output in a goroutine
		go func() {
			defer close(s.done)
			scanner := bufio.NewScanner(stdout)
			scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
		scan:
			for scanner.Scan() {
				msg := logMsg{serviceName: s.Name, line: scanner.Text(), time: time.Now()}
				select {
				case logCh <- msg:
				case <-ctx.Done():
					break scan
				default:
					// Channel full — drop line to avoid blocking
				}
			}
			// Process exited — determine final status.
			// These sends use a timeout: they are few and important,
			// but must not block forever if the consumer is gone.
			err := s.cmd.Wait()
			finalStatus := Stopped
			if err != nil && ctx.Err() == nil {
				finalStatus = Error
				trySendTimeout(logCh, logMsg{serviceName: s.Name, line: fmt.Sprintf("[vista] process exited with error: %v", err), time: time.Now()})
			} else {
				trySendTimeout(logCh, logMsg{serviceName: s.Name, line: "[vista] process stopped", time: time.Now()})
			}
			trySendTimeout(logCh, logMsg{serviceName: s.Name, line: fmt.Sprintf("[vista] status: %s", finalStatus), time: time.Now()})
			// Status update is critical — use a longer timeout
			trySendTimeout(logCh, serviceStatusMsg{serviceName: s.Name, status: finalStatus})
		}()

		return serviceStatusMsg{serviceName: s.Name, status: Running, pid: pid}
	}
}

// trySendTimeout attempts to send a message on the channel with a 5-second timeout.
// Used for post-exit lifecycle messages that are important but must not block forever.
func trySendTimeout(ch chan<- tea.Msg, msg tea.Msg) {
	select {
	case ch <- msg:
	case <-time.After(5 * time.Second):
	}
}

// Stop terminates the service and its process group.
func (s *Service) Stop() tea.Cmd {
	if (s.Status != Running && s.Status != Stopping) || s.PID == 0 {
		return nil
	}

	pid := s.PID
	name := s.Name

	s.mu.Lock()
	done := s.done
	cancel := s.cancel
	s.mu.Unlock()

	if cancel != nil {
		cancel()
	}

	return func() tea.Msg {
		// Kill the process group with SIGTERM
		_ = syscall.Kill(-pid, syscall.SIGTERM)

		// Wait briefly, then SIGKILL as fallback — exits early if process dies
		go func() {
			select {
			case <-done:
				// Process already exited, no need for SIGKILL
			case <-time.After(2 * time.Second):
				_ = syscall.Kill(-pid, syscall.SIGKILL)
			}
		}()

		return serviceStatusMsg{serviceName: name, status: Stopping}
	}
}
