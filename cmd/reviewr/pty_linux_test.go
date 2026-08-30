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

	first := startPTYReviewr(t, repository, stateHome, 80, 24)
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

	first.resize(t, 60, 12)
	first.waitFor(t, "Scratch")
	first.resetOutput()
	first.write(t, "\x1b")
	first.waitFor(t, "files")
	first.resetOutput()
	first.write(t, "\x1b")
	first.waitFor(t, "Scratch")
	first.waitFor(t, "wide 界")

	second := startPTYReviewr(t, repository, stateHome, 60, 12)
	t.Cleanup(second.stop)
	second.waitFor(t, "files")
	second.resetOutput()
	second.write(t, "\x1b")
	second.waitFor(t, "read-only")
	second.write(t, "x")
	second.waitFor(t, "another reviewr is editing")
}

func TestPTYAutomaticallyRefreshesChangedFileWithoutMovingTheUser(t *testing.T) {
	if testing.Short() {
		t.Skip("PTY smoke test")
	}
	repository := t.TempDir()
	mustRunGit(t, repository, "init", "-q")
	mustRunGit(t, repository, "config", "user.name", "Reviewr Test")
	mustRunGit(t, repository, "config", "user.email", "reviewr@example.invalid")
	mustWriteFixture(t, repository, "main.go", []byte("package main\nconst Value = \"committed0\"\n"))
	mustRunGit(t, repository, "add", "--", ".")
	mustRunGit(t, repository, "commit", "-qm", "fixture")
	mustWriteFixture(t, repository, "main.go", []byte("package main\nconst Value = \"poll-first\"\n"))

	session := startPTYReviewr(t, repository, t.TempDir(), 80, 16)
	t.Cleanup(session.stop)
	session.waitFor(t, "poll-first")
	session.write(t, "\t")
	session.resetOutput()
	mustWriteFixture(t, repository, "main.go", []byte("package main\nconst Value = \"poll-other\"\n"))
	// Bubble Tea emits only the changed cell run ("other"); the existing
	// "poll-" prefix remains on the terminal screen.
	session.waitFor(t, "other")
}

func TestPTYReviewLedgerReconciliation(t *testing.T) {
	if testing.Short() {
		t.Skip("PTY smoke test")
	}
	repository := t.TempDir()
	mustRunGit(t, repository, "init", "-q")
	mustRunGit(t, repository, "config", "user.name", "Reviewr Test")
	mustRunGit(t, repository, "config", "user.email", "reviewr@example.invalid")
	mustWriteFixture(t, repository, "src/a.go", []byte("package src\nconst A = 0\n"))
	mustWriteFixture(t, repository, "src/b.go", []byte("package src\nconst B = 0\n"))
	mustWriteFixture(t, repository, "binary.bin", []byte{0, 1})
	mustWriteFixture(t, repository, "root.go", []byte("package root\nconst Version = 0\n"))
	mustRunGit(t, repository, "add", "--", ".")
	mustRunGit(t, repository, "commit", "-qm", "fixture")

	mustWriteFixture(t, repository, "src/a.go", []byte("package src\nconst A = 1\n"))
	mustWriteFixture(t, repository, "src/b.go", []byte("package src\nconst B = 1\n"))
	mustWriteFixture(t, repository, "binary.bin", []byte{0, 2})
	mustWriteFixture(t, repository, "root.go", []byte("package root\nconst Version = 1\n"))
	stateHome := t.TempDir()

	first := startPTYReviewr(t, repository, stateHome, 80, 24)
	t.Cleanup(first.stop)
	first.waitFor(t, "0/2")
	first.waitFor(t, "Binary file")
	first.resetOutput()
	first.write(t, "x")
	waitForReviewReceipts(t, stateHome, 1)
	first.waitFor(t, "[x]")

	// A changed binary checkpoint has no retained body, so refresh must be
	// conservative rather than inventing an incremental comparison.
	mustWriteFixture(t, repository, "binary.bin", []byte{0, 3})
	first.resetOutput()
	first.write(t, "r")
	first.waitFor(t, "[!]")
	first.resetOutput()
	first.write(t, "x")
	waitForReviewReceipts(t, stateHome, 2)
	first.waitFor(t, "[x]")

	// X expands the initially collapsed src ancestor. At 80 columns the
	// navigator is 26 cells wide; SGR column 23 is the separator cell of the
	// painted four-cell review target for src/a.go on terminal row 4.
	first.resetOutput()
	first.write(t, "X")
	first.waitFor(t, "src/a.go")
	first.resetOutput()
	first.write(t, "\x1b[<0;23;4M")
	waitForReviewReceipts(t, stateHome, 3)

	first.resetOutput()
	first.write(t, "X")
	first.waitFor(t, "const B")
	first.write(t, "x")
	waitForReviewReceipts(t, stateHome, 4)

	first.resetOutput()
	first.write(t, "X")
	first.waitFor(t, "Version")
	first.write(t, "x")
	waitForReviewReceipts(t, stateHome, 5)

	// A retained text checkpoint advances honestly to Updated. Diff mode opens
	// since reviewed, R is bounds-only, and reader-focused x marks that edge.
	mustWriteFixture(t, repository, "root.go", []byte("package root\nconst Version = 2\n"))
	first.resetOutput()
	first.write(t, "r")
	first.waitFor(t, "[+]")
	first.resetOutput()
	first.write(t, "5")
	first.waitFor(t, "since reviewed")
	first.write(t, "\t")
	first.resetOutput()
	first.write(t, "R")
	first.waitFor(t, "full comparison")
	first.resetOutput()
	first.write(t, "R")
	first.waitFor(t, "since reviewed")
	first.write(t, "x")
	waitForReviewReceipts(t, stateHome, 6)
	first.waitFor(t, "[x]")

	// Git and Scratch keep their own input and place state; returning to Files
	// recovers the same reviewed comparison. The narrow frame keeps the badge.
	first.resetOutput()
	first.write(t, "1")
	first.waitFor(t, "commits")
	first.write(t, "xRX")
	first.resetOutput()
	first.write(t, "\x1b")
	first.waitFor(t, "Scratch")
	first.resetOutput()
	first.write(t, "1")
	first.waitFor(t, "commits")
	first.resetOutput()
	first.write(t, "1")
	first.waitFor(t, "[x]")
	first.resize(t, 60, 12)
	first.resetOutput()
	first.write(t, "r")
	first.waitFor(t, "[x]")
	first.stop()

	restarted := startPTYReviewr(t, repository, stateHome, 60, 12)
	t.Cleanup(restarted.stop)
	restarted.waitFor(t, "[x]")
	restarted.waitFor(t, "2/2")
}

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
