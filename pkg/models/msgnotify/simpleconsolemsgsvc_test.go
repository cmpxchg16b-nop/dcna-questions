package msgnotify

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"testing"
	"time"
)

// TestSimpleConsoleMessagingService_HelperProcess is not a real test. When
// re-executed as a subprocess with GO_WANT_HELPER_PROCESS=1, it invokes Send
// with parameters taken from the environment and exits: 0 on success, 1 if
// Send returned an error. This lets the parent test capture what Send writes
// to the real os.Stdout and os.Stderr.
func TestSimpleConsoleMessagingService_HelperProcess(t *testing.T) {
	if os.Getenv("GO_WANT_HELPER_PROCESS") != "1" {
		t.Skip("not running as a helper process")
	}

	created, err := strconv.ParseInt(os.Getenv("HELPER_MSG_CREATED"), 10, 64)
	if err != nil {
		fmt.Fprintf(os.Stderr, "bad HELPER_MSG_CREATED: %v\n", err)
		os.Exit(2)
	}
	level, err := strconv.Atoi(os.Getenv("HELPER_MSG_LEVEL"))
	if err != nil {
		fmt.Fprintf(os.Stderr, "bad HELPER_MSG_LEVEL: %v\n", err)
		os.Exit(2)
	}

	replyTo := AddrId{
		AddressFamily: MsgNotifyAddrFamily(os.Getenv("HELPER_REPLY_FAMILY")),
		Address:       os.Getenv("HELPER_REPLY_ADDR"),
	}
	to := AddrId{
		AddressFamily: MsgNotifyAddrFamily(os.Getenv("HELPER_TO_FAMILY")),
		Address:       os.Getenv("HELPER_TO_ADDR"),
	}
	msg := Msg{
		Id:      os.Getenv("HELPER_MSG_ID"),
		Created: created,
		Title:   os.Getenv("HELPER_MSG_TITLE"),
		Level:   MessageLevel(level),
		Text:    os.Getenv("HELPER_MSG_TEXT"),
	}

	svc := SimpleConsoleMessagingService{}
	if err := svc.Send(context.Background(), replyTo, to, msg); err != nil {
		fmt.Fprintf(os.Stderr, "send failed: %v\n", err)
		os.Exit(1)
	}
	os.Exit(0)
}

// runConsoleHelper re-executes the test binary as a subprocess running only
// the helper process "test", and captures its stdout and stderr separately.
func runConsoleHelper(t *testing.T, env map[string]string) (stdout string, stderr string, err error) {
	t.Helper()

	cmd := exec.Command(os.Args[0], "-test.run=^TestSimpleConsoleMessagingService_HelperProcess$")
	cmd.Env = append(os.Environ(), "GO_WANT_HELPER_PROCESS=1")
	for k, v := range env {
		cmd.Env = append(cmd.Env, k+"="+v)
	}

	var outBuf, errBuf bytes.Buffer
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf
	err = cmd.Run()
	return outBuf.String(), errBuf.String(), err
}

func helperEnv(replyTo AddrId, to AddrId, msg Msg) map[string]string {
	return map[string]string{
		"HELPER_REPLY_FAMILY": string(replyTo.AddressFamily),
		"HELPER_REPLY_ADDR":   replyTo.Address,
		"HELPER_TO_FAMILY":    string(to.AddressFamily),
		"HELPER_TO_ADDR":      to.Address,
		"HELPER_MSG_ID":       msg.Id,
		"HELPER_MSG_CREATED":  strconv.FormatInt(msg.Created, 10),
		"HELPER_MSG_TITLE":    msg.Title,
		"HELPER_MSG_LEVEL":    strconv.Itoa(int(msg.Level)),
		"HELPER_MSG_TEXT":     msg.Text,
	}
}

func expectedLine(replyTo AddrId, msg Msg) string {
	return fmt.Sprintf("At %s, Level: %d, From: %s:%s Message: %s\n",
		time.UnixMilli(msg.Created).Format(time.RFC3339),
		msg.Level,
		replyTo.AddressFamily, replyTo.Address,
		msg.Text,
	)
}

func TestSimpleConsoleMessagingService_AreYou(t *testing.T) {
	svc := SimpleConsoleMessagingService{}

	for _, addr := range []AddrId{
		{AddressFamily: MsgNotifyAddrFamilyConsole, Address: WellKnownAddrConsoleStdout},
		{AddressFamily: MsgNotifyAddrFamilyConsole, Address: WellKnownAddrConsoleStderr},
	} {
		if !svc.AreYou(addr) {
			t.Errorf("AreYou(%v) = false, want true", addr)
		}
	}
	for _, addr := range []AddrId{
		{AddressFamily: MsgNotifyAddrFamilyConsole, Address: "/dev/tty"},
		{AddressFamily: MsgNotifyAddrFamilyEmail, Address: "user@example.com"},
		{AddressFamily: MsgNotifyAddrFamilyService, Address: WellKnownAddrServiceOnMemoryExamTrackingServer},
	} {
		if svc.AreYou(addr) {
			t.Errorf("AreYou(%v) = true, want false", addr)
		}
	}
}

func TestSimpleConsoleMessagingService_Send(t *testing.T) {
	const created = int64(1735689600000) // 2025-01-01T00:00:00Z

	t.Run("writes to stdout for the stdout address", func(t *testing.T) {
		replyTo := AddrId{AddressFamily: MsgNotifyAddrFamilyEmail, Address: "alice@example.com"}
		to := AddrId{AddressFamily: MsgNotifyAddrFamilyConsole, Address: WellKnownAddrConsoleStdout}
		msg := Msg{Id: "msg-1", Created: created, Title: "t", Level: MessageLevelImportant, Text: "hello stdout"}

		stdout, stderr, err := runConsoleHelper(t, helperEnv(replyTo, to, msg))
		if err != nil {
			t.Fatalf("helper process failed: %v (stderr: %q)", err, stderr)
		}
		if want := expectedLine(replyTo, msg); stdout != want {
			t.Errorf("stdout = %q, want %q", stdout, want)
		}
		if stderr != "" {
			t.Errorf("stderr = %q, want empty", stderr)
		}
	})

	t.Run("writes to stderr for the stderr address", func(t *testing.T) {
		replyTo := AddrId{AddressFamily: MsgNotifyAddrFamilyConsole, Address: WellKnownAddrConsoleStdout}
		to := AddrId{AddressFamily: MsgNotifyAddrFamilyConsole, Address: WellKnownAddrConsoleStderr}
		msg := Msg{Id: "msg-2", Created: created, Title: "t", Level: MessageLevelCommon, Text: "hello stderr"}

		stdout, stderr, err := runConsoleHelper(t, helperEnv(replyTo, to, msg))
		if err != nil {
			t.Fatalf("helper process failed: %v (stderr: %q)", err, stderr)
		}
		if want := expectedLine(replyTo, msg); stderr != want {
			t.Errorf("stderr = %q, want %q", stderr, want)
		}
		if stdout != "" {
			t.Errorf("stdout = %q, want empty", stdout)
		}
	})

	t.Run("tolerates reply-to address family mismatch", func(t *testing.T) {
		// replyTo is in the service family while the destination is console;
		// Send must still succeed.
		replyTo := AddrId{AddressFamily: MsgNotifyAddrFamilyService, Address: "exam-server"}
		to := AddrId{AddressFamily: MsgNotifyAddrFamilyConsole, Address: WellKnownAddrConsoleStdout}
		msg := Msg{Id: "msg-3", Created: created, Title: "t", Level: MessageLevelCommon, Text: "mismatch ok"}

		stdout, stderr, err := runConsoleHelper(t, helperEnv(replyTo, to, msg))
		if err != nil {
			t.Fatalf("helper process failed: %v (stderr: %q)", err, stderr)
		}
		if want := expectedLine(replyTo, msg); stdout != want {
			t.Errorf("stdout = %q, want %q", stdout, want)
		}
	})

	t.Run("rejects non-console destination address family", func(t *testing.T) {
		replyTo := AddrId{AddressFamily: MsgNotifyAddrFamilyConsole, Address: WellKnownAddrConsoleStdout}
		to := AddrId{AddressFamily: MsgNotifyAddrFamilyEmail, Address: "bob@example.com"}
		msg := Msg{Id: "msg-4", Created: created, Title: "t", Level: MessageLevelImportant, Text: "nope"}

		stdout, stderr, err := runConsoleHelper(t, helperEnv(replyTo, to, msg))
		if err == nil {
			t.Fatal("helper process succeeded, want Send to fail")
		}
		if want := `unsupported destination address family "email"`; !bytes.Contains([]byte(stderr), []byte(want)) {
			t.Errorf("stderr = %q, want it to contain %q", stderr, want)
		}
		if stdout != "" {
			t.Errorf("stdout = %q, want empty", stdout)
		}
	})

	t.Run("rejects unknown console address", func(t *testing.T) {
		replyTo := AddrId{AddressFamily: MsgNotifyAddrFamilyConsole, Address: WellKnownAddrConsoleStdout}
		to := AddrId{AddressFamily: MsgNotifyAddrFamilyConsole, Address: "/dev/tty"}
		msg := Msg{Id: "msg-5", Created: created, Title: "t", Level: MessageLevelCommon, Text: "nope"}

		_, stderr, err := runConsoleHelper(t, helperEnv(replyTo, to, msg))
		if err == nil {
			t.Fatal("helper process succeeded, want Send to fail")
		}
		if want := `unknown console address "/dev/tty"`; !bytes.Contains([]byte(stderr), []byte(want)) {
			t.Errorf("stderr = %q, want it to contain %q", stderr, want)
		}
	})
}

// captureStdoutStderr replaces os.Stdout and os.Stderr with pipes for the
// duration of fn, restores them afterwards, and returns whatever fn wrote to
// each stream along with fn's error. Callers must not run such tests in
// parallel, since the process-global streams are swapped.
func captureStdoutStderr(t *testing.T, fn func() error) (stdout string, stderr string, err error) {
	t.Helper()

	rOut, wOut, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe (stdout): %v", err)
	}
	defer rOut.Close()

	rErr, wErr, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe (stderr): %v", err)
	}
	defer rErr.Close()

	oldOut, oldErr := os.Stdout, os.Stderr
	os.Stdout, os.Stderr = wOut, wErr
	defer func() { os.Stdout, os.Stderr = oldOut, oldErr }()

	fnErr := fn()

	// Close the write ends so ReadAll sees EOF.
	if err := wOut.Close(); err != nil {
		t.Fatalf("closing stdout pipe writer: %v", err)
	}
	if err := wErr.Close(); err != nil {
		t.Fatalf("closing stderr pipe writer: %v", err)
	}

	outBytes, err := io.ReadAll(rOut)
	if err != nil {
		t.Fatalf("reading stdout pipe: %v", err)
	}
	errBytes, err := io.ReadAll(rErr)
	if err != nil {
		t.Fatalf("reading stderr pipe: %v", err)
	}
	return string(outBytes), string(errBytes), fnErr
}

func TestSimpleConsoleMessagingService_Send_Pipes(t *testing.T) {
	const created = int64(1735689600000) // 2025-01-01T00:00:00Z

	svc := SimpleConsoleMessagingService{}

	t.Run("stdout destination writes only to stdout", func(t *testing.T) {
		replyTo := AddrId{AddressFamily: MsgNotifyAddrFamilyEmail, Address: "alice@example.com"}
		to := AddrId{AddressFamily: MsgNotifyAddrFamilyConsole, Address: WellKnownAddrConsoleStdout}
		msg := Msg{Id: "pipe-1", Created: created, Title: "t", Level: MessageLevelImportant, Text: "via pipe"}

		stdout, stderr, err := captureStdoutStderr(t, func() error {
			return svc.Send(context.Background(), replyTo, to, msg)
		})
		if err != nil {
			t.Fatalf("Send failed: %v", err)
		}
		if want := expectedLine(replyTo, msg); stdout != want {
			t.Errorf("stdout = %q, want %q", stdout, want)
		}
		if stderr != "" {
			t.Errorf("stderr = %q, want empty", stderr)
		}
	})

	t.Run("stderr destination writes only to stderr", func(t *testing.T) {
		replyTo := AddrId{AddressFamily: MsgNotifyAddrFamilyConsole, Address: WellKnownAddrConsoleStdout}
		to := AddrId{AddressFamily: MsgNotifyAddrFamilyConsole, Address: WellKnownAddrConsoleStderr}
		msg := Msg{Id: "pipe-2", Created: created, Title: "t", Level: MessageLevelCommon, Text: "via pipe"}

		stdout, stderr, err := captureStdoutStderr(t, func() error {
			return svc.Send(context.Background(), replyTo, to, msg)
		})
		if err != nil {
			t.Fatalf("Send failed: %v", err)
		}
		if want := expectedLine(replyTo, msg); stderr != want {
			t.Errorf("stderr = %q, want %q", stderr, want)
		}
		if stdout != "" {
			t.Errorf("stdout = %q, want empty", stdout)
		}
	})

	t.Run("tolerates reply-to address family mismatch", func(t *testing.T) {
		replyTo := AddrId{AddressFamily: MsgNotifyAddrFamilyService, Address: "exam-server"}
		to := AddrId{AddressFamily: MsgNotifyAddrFamilyConsole, Address: WellKnownAddrConsoleStdout}
		msg := Msg{Id: "pipe-3", Created: created, Title: "t", Level: MessageLevelCommon, Text: "mismatch ok"}

		stdout, _, err := captureStdoutStderr(t, func() error {
			return svc.Send(context.Background(), replyTo, to, msg)
		})
		if err != nil {
			t.Fatalf("Send failed: %v", err)
		}
		if want := expectedLine(replyTo, msg); stdout != want {
			t.Errorf("stdout = %q, want %q", stdout, want)
		}
	})

	t.Run("rejects non-console destination without writing anything", func(t *testing.T) {
		replyTo := AddrId{AddressFamily: MsgNotifyAddrFamilyConsole, Address: WellKnownAddrConsoleStdout}
		to := AddrId{AddressFamily: MsgNotifyAddrFamilyEmail, Address: "bob@example.com"}
		msg := Msg{Id: "pipe-4", Created: created, Title: "t", Level: MessageLevelImportant, Text: "nope"}

		stdout, stderr, err := captureStdoutStderr(t, func() error {
			return svc.Send(context.Background(), replyTo, to, msg)
		})
		if err == nil {
			t.Fatal("Send succeeded, want error")
		}
		if want := `unsupported destination address family "email"`; !strings.Contains(err.Error(), want) {
			t.Errorf("error = %q, want it to contain %q", err, want)
		}
		if stdout != "" || stderr != "" {
			t.Errorf("stdout = %q, stderr = %q, want both empty", stdout, stderr)
		}
	})

	t.Run("rejects unknown console address without writing anything", func(t *testing.T) {
		replyTo := AddrId{AddressFamily: MsgNotifyAddrFamilyConsole, Address: WellKnownAddrConsoleStdout}
		to := AddrId{AddressFamily: MsgNotifyAddrFamilyConsole, Address: "/dev/tty"}
		msg := Msg{Id: "pipe-5", Created: created, Title: "t", Level: MessageLevelCommon, Text: "nope"}

		stdout, stderr, err := captureStdoutStderr(t, func() error {
			return svc.Send(context.Background(), replyTo, to, msg)
		})
		if err == nil {
			t.Fatal("Send succeeded, want error")
		}
		if want := `unknown console address "/dev/tty"`; !strings.Contains(err.Error(), want) {
			t.Errorf("error = %q, want it to contain %q", err, want)
		}
		if stdout != "" || stderr != "" {
			t.Errorf("stdout = %q, stderr = %q, want both empty", stdout, stderr)
		}
	})
}
