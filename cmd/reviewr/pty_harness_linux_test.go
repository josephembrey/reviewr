package main

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/charmbracelet/x/ansi"
	"golang.org/x/sys/unix"
)

func mustRunGit(t *testing.T, repository string, args ...string) {
	t.Helper()
	commandArgs := append([]string{"-C", repository}, args...)
	command := exec.Command("git", commandArgs...)
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("git %s: %v: %s", strings.Join(args, " "), err, bytes.TrimSpace(output))
	}
}

func mustWriteFixture(t *testing.T, repository, path string, content []byte) {
	t.Helper()
	fullPath := filepath.Join(repository, filepath.FromSlash(path))
	if err := os.MkdirAll(filepath.Dir(fullPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(fullPath, content, 0o644); err != nil {
		t.Fatal(err)
	}
}

func waitForReviewReceipts(t *testing.T, stateHome string, want int) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	pattern := filepath.Join(stateHome, "reviewr", "reviews", "*.json")
	for time.Now().Before(deadline) {
		paths, _ := filepath.Glob(pattern)
		for _, path := range paths {
			data, err := os.ReadFile(path)
			if err == nil && bytes.Count(data, []byte(`"sequence"`)) >= want {
				return
			}
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("review state never persisted %d receipts beneath %s", want, stateHome)
}

func waitForSessionValue(t *testing.T, stateHome, value string) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	pattern := filepath.Join(stateHome, "reviewr", "sessions", "*.json")
	for time.Now().Before(deadline) {
		paths, _ := filepath.Glob(pattern)
		for _, path := range paths {
			data, err := os.ReadFile(path)
			if err == nil && bytes.Contains(data, []byte(value)) {
				return
			}
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("worktree session never contained %q beneath %s", value, stateHome)
}

type ptySession struct {
	master  *os.File
	cmd     *exec.Cmd
	output  lockedBuffer
	done    chan struct{}
	waitMu  sync.Mutex
	waitErr error
	once    sync.Once
}

func startPTYReviewr(t *testing.T, repository, stateHome string, width, height uint16) *ptySession {
	t.Helper()
	masterFD, err := unix.Open("/dev/ptmx", unix.O_RDWR|unix.O_NOCTTY, 0)
	if err != nil {
		t.Fatal(err)
	}
	master := os.NewFile(uintptr(masterFD), "/dev/ptmx")
	cleanupMaster := true
	defer func() {
		if cleanupMaster {
			_ = master.Close()
		}
	}()
	if err := unix.IoctlSetPointerInt(masterFD, unix.TIOCSPTLCK, 0); err != nil {
		t.Fatal(err)
	}
	ptyNumber, err := unix.IoctlGetInt(masterFD, unix.TIOCGPTN)
	if err != nil {
		t.Fatal(err)
	}
	if err := unix.IoctlSetWinsize(masterFD, unix.TIOCSWINSZ, &unix.Winsize{Col: width, Row: height}); err != nil {
		t.Fatal(err)
	}
	slave, err := os.OpenFile(filepath.Join("/dev/pts", fmt.Sprint(ptyNumber)), os.O_RDWR|syscall.O_NOCTTY, 0)
	if err != nil {
		t.Fatal(err)
	}
	command := exec.Command(os.Args[0], "-test.run=^TestPTYHelperProcess$")
	command.Env = append(os.Environ(),
		ptyHelperEnvironment+"=1",
		"REVIEWR_PTY_REPOSITORY="+repository,
		"XDG_STATE_HOME="+stateHome,
		"TERM=xterm-256color",
	)
	command.Stdin = slave
	command.Stdout = slave
	command.Stderr = slave
	command.SysProcAttr = &syscall.SysProcAttr{Setsid: true, Setctty: true, Ctty: 0}
	if err := command.Start(); err != nil {
		_ = slave.Close()
		t.Fatal(err)
	}
	_ = slave.Close()
	cleanupMaster = false
	session := &ptySession{master: master, cmd: command, done: make(chan struct{})}
	go func() {
		_, _ = io.Copy(&session.output, master)
	}()
	go func() {
		err := command.Wait()
		session.waitMu.Lock()
		session.waitErr = err
		session.waitMu.Unlock()
		close(session.done)
	}()
	return session
}

func (session *ptySession) write(t *testing.T, value string) {
	t.Helper()
	if _, err := io.WriteString(session.master, value); err != nil {
		t.Fatal(err)
	}
}

func (session *ptySession) resize(t *testing.T, width, height uint16) {
	t.Helper()
	if err := unix.IoctlSetWinsize(int(session.master.Fd()), unix.TIOCSWINSZ, &unix.Winsize{Col: width, Row: height}); err != nil {
		t.Fatal(err)
	}
	if err := session.cmd.Process.Signal(syscall.SIGWINCH); err != nil {
		t.Fatal(err)
	}
}

func (session *ptySession) waitFor(t *testing.T, value string) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if containsPTYText(session.output.String(), value) {
			return
		}
		select {
		case <-session.done:
			t.Fatalf("reviewr exited before %q: %v\n%s", value, session.err(), session.output.String())
		default:
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("PTY output never contained %q:\n%s", value, session.output.String())
}

func containsPTYText(output, value string) bool {
	return strings.Contains(ansi.Strip(output), value)
}

func TestPTYTextAssertionsIgnoreANSIStyles(t *testing.T) {
	output := "\x1b[2;3H\x1b[34mconst\x1b[m B\x1b[K"
	if !containsPTYText(output, "const B") {
		t.Fatalf("styled PTY output did not expose its visible text: %q", output)
	}
}

func (session *ptySession) resetOutput() { session.output.Reset() }

func (session *ptySession) stop() {
	session.once.Do(func() {
		_, _ = io.WriteString(session.master, "\x03")
		select {
		case <-session.done:
		case <-time.After(2 * time.Second):
			_ = session.cmd.Process.Kill()
			<-session.done
		}
		_ = session.master.Close()
	})
}

func (session *ptySession) err() error {
	session.waitMu.Lock()
	defer session.waitMu.Unlock()
	return session.waitErr
}

type lockedBuffer struct {
	mu     sync.Mutex
	buffer bytes.Buffer
}

func (buffer *lockedBuffer) Write(value []byte) (int, error) {
	buffer.mu.Lock()
	defer buffer.mu.Unlock()
	return buffer.buffer.Write(value)
}

func (buffer *lockedBuffer) String() string {
	buffer.mu.Lock()
	defer buffer.mu.Unlock()
	return buffer.buffer.String()
}

func (buffer *lockedBuffer) Reset() {
	buffer.mu.Lock()
	defer buffer.mu.Unlock()
	buffer.buffer.Reset()
}
