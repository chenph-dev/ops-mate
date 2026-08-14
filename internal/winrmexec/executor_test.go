package winrmexec

import (
	"context"
	"testing"

	"ops-mate/internal/sshexec"
)

// stubRunner implements the runner interface, returning preset output.
type stubRunner struct {
	stdout string
	stderr string
	code   int
	err    error
	gotCmd string
}

func (r *stubRunner) RunPSWithContext(_ context.Context, command string) (string, string, int, error) {
	r.gotCmd = command
	return r.stdout, r.stderr, r.code, r.err
}

// blockingRunner blocks until ctx is cancelled, then returns, exercising the
// ctx.Done() select branches in Exec's emit goroutine.
type blockingRunner struct{}

func (blockingRunner) RunPSWithContext(ctx context.Context, _ string) (string, string, int, error) {
	<-ctx.Done()
	return "", "", 0, nil
}

func testExecutor(r runner) *Executor {
	return &Executor{host: Host{Addr: "10.0.0.1", Port: 5985, User: "admin", Secret: "x"},
		newClient: func(Host) (runner, error) { return r, nil }}
}

func drain(t *testing.T, ch <-chan sshexec.Line) []sshexec.Line {
	t.Helper()
	var lines []sshexec.Line
	for ln := range ch {
		lines = append(lines, ln)
	}
	return lines
}

func TestExec_LineSplittingAndExitCode(t *testing.T) {
	r := &stubRunner{stdout: "line1\nline2\n", stderr: "err1\n", code: 3}
	ch, err := testExecutor(r).Exec(context.Background(), "whoami")
	if err != nil {
		t.Fatalf("Exec: %v", err)
	}
	lines := drain(t, ch)

	if len(lines) != 4 {
		t.Fatalf("expected 4 lines, got %d: %+v", len(lines), lines)
	}
	if lines[0] != (sshexec.Line{Stream: "stdout", Text: "line1"}) ||
		lines[1] != (sshexec.Line{Stream: "stdout", Text: "line2"}) ||
		lines[2] != (sshexec.Line{Stream: "stderr", Text: "err1"}) ||
		lines[3] != (sshexec.Line{Stream: "exit", Text: "exit_code=3"}) {
		t.Errorf("line order/content wrong: %+v", lines)
	}
	if r.gotCmd != "whoami" {
		t.Errorf("should pass command verbatim, got %q", r.gotCmd)
	}
}

func TestExec_RunErrorEmitsErrorLine(t *testing.T) {
	r := &stubRunner{stdout: "partial", err: context.DeadlineExceeded}
	ch, err := testExecutor(r).Exec(context.Background(), "bad")
	if err != nil {
		t.Fatalf("Exec: %v", err)
	}
	lines := drain(t, ch)

	var sawError bool
	for _, ln := range lines {
		if ln.Stream == "error" && ln.Text == context.DeadlineExceeded.Error() {
			sawError = true
		}
		if ln.Stream == "exit" {
			t.Error("run error should not emit exit line")
		}
	}
	if !sawError {
		t.Errorf("should emit error line, got %+v", lines)
	}
}

func TestExec_NewClientErrorReturnsSyncError(t *testing.T) {
	e := &Executor{host: Host{Addr: "x"}, newClient: func(Host) (runner, error) {
		return nil, context.DeadlineExceeded
	}}
	if _, err := e.Exec(context.Background(), "whoami"); err == nil {
		t.Error("newClient failure should return a sync error")
	}
}

func TestHttpsForPort(t *testing.T) {
	if !httpsForPort(5986) {
		t.Error("5986 should be HTTPS")
	}
	if httpsForPort(5985) {
		t.Error("5985 should be HTTP")
	}
	if httpsForPort(22) {
		t.Error("22 should be HTTP")
	}
}

func TestExec_ContextCancellationClosesChannel(t *testing.T) {
	e := testExecutor(blockingRunner{})
	ctx, cancel := context.WithCancel(context.Background())
	ch, err := e.Exec(ctx, "long")
	if err != nil {
		t.Fatalf("Exec: %v", err)
	}
	cancel()
	// cancellation must cause the emit goroutine to stop and close the channel.
	lines := drain(t, ch)
	if len(lines) != 0 {
		t.Errorf("expected no lines on cancellation, got %+v", lines)
	}
}

func TestSplitLines(t *testing.T) {
	cases := []struct {
		in   string
		want []string
	}{
		{"a\r\nb\r\n", []string{"a", "b"}},
		{"a\n\nb\n", []string{"a", "", "b"}},
		{"", []string{}},
		{"single", []string{"single"}},
	}
	for _, c := range cases {
		got := splitLines(c.in)
		if len(got) != len(c.want) {
			t.Errorf("splitLines(%q) = %v, want %v", c.in, got, c.want)
			continue
		}
		for i := range got {
			if got[i] != c.want[i] {
				t.Errorf("splitLines(%q) = %v, want %v", c.in, got, c.want)
				break
			}
		}
	}
}
