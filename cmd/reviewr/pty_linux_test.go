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

	"golang.org/x/sys/unix"
)

const ptyHelperEnvironment = "REVIEWR_PTY_HELPER"

func TestPTYHelperProcess(t *testing.T) {
	if os.Getenv(ptyHelperEnvironment) != "1" {
		return
	}
	if err := run([]string{os.Getenv("REVIEWR_PTY_REPOSITORY")}); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	os.Exit(0)
}

func TestPTYScratchEditingPersistenceAndLocking(t *testing.T) {
	if testing.Short() {
		t.Skip("PTY smoke test")
	}
	repository := t.TempDir()
	command := exec.Command("git", "init", "-q", repository)
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("git init: %v: %s", err, bytes.TrimSpace(output))
	}
	stateHome := t.TempDir()

	first := startPTYReviewr(t, repository, stateHome, 80, 20)
	t.Cleanup(first.stop)
	first.waitFor(t, "files")
	first.resetOutput()
	first.write(t, "\x1b")
	first.waitFor(t, "Scratch")
	first.waitFor(t, "saved")

	first.resetOutput()
	first.write(t, "hello")
	first.write(t, "\x1b[200~\nwide 界\tpaste\x1b[201~")
	first.waitFor(t, "Col 14")
	first.waitFor(t, "wide 界")

	// SGR mouse: click, drag-select, wheel, and the painted scrollbar lane.
	first.write(t, "\x1b[<0;2;3M\x1b[<0;2;3mX")
	first.waitFor(t, "X")
	first.write(t, "\x1b[<0;1;3M\x1b[<32;4;3M\x1b[<0;4;3mZ")
	first.waitFor(t, "Z")
	first.write(t, "\x1b[200~"+strings.Repeat("line\n", 35)+"\x1b[201~")
	first.write(t, "\x1b[<65;2;4M")
	first.write(t, "\x1b[<0;80;5M\x1b[<32;80;12M\x1b[<0;80;12m")

	first.resize(t, 64, 15)
	first.waitFor(t, "Scratch")
	first.resetOutput()
	first.write(t, "\x1b")
	first.waitFor(t, "files")
	first.resetOutput()
	first.write(t, "\x1b")
	first.waitFor(t, "Scratch")
	first.waitFor(t, "wide 界")

	second := startPTYReviewr(t, repository, stateHome, 80, 20)
	t.Cleanup(second.stop)
	second.waitFor(t, "files")
	second.resetOutput()
	second.write(t, "\x1b")
	second.waitFor(t, "read-only")
	second.write(t, "x")
	second.waitFor(t, "another reviewr is editing")
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
	cleanupMaster := true
	defer func() {
		if cleanupMaster {
			_ = unix.Close(masterFD)
		}
	}()
	if err := unix.IoctlSetPointerInt(masterFD, unix.TIOCSPTLCK, 0); err != nil {
		t.Fatal(err)
	}
	ptyNumber, err := unix.IoctlGetInt(masterFD, unix.TIOCGPTN)
	if err != nil {
		t.Fatal(err)
	}
	master := os.NewFile(uintptr(masterFD), "/dev/ptmx")
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
		if strings.Contains(session.output.String(), value) {
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
