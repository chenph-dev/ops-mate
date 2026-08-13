// Package winrmexec provides a WinRM command executor (implements sshexec.Exec for protocol-agnostic reuse).
package winrmexec

import (
	"context"
	"fmt"
	"strings"

	"github.com/masterzen/winrm"

	"ops-mate/internal/sshexec"
)

// Host describes a WinRM target (password auth only).
type Host struct {
	Addr   string
	Port   int    // 5985 = HTTP, 5986 = HTTPS (skip cert verify)
	User   string
	Secret string // plaintext password
}

// runner abstracts the WinRM client's PowerShell execution for test stubbing.
// *winrm.Client's RunPSWithContext naturally satisfies this interface.
type runner interface {
	RunPSWithContext(ctx context.Context, command string) (stdout, stderr string, exitCode int, err error)
}

// Executor implements sshexec.Exec (reuses sshexec.Line as the line output type).
type Executor struct {
	host      Host
	newClient func(Host) (runner, error)
}

// NewExecutor constructs an Executor (defaults to the masterzen/winrm client).
func NewExecutor(h Host) *Executor {
	return &Executor{host: h, newClient: defaultNewClient}
}

// Exec runs a command, returning a line stream channel (closed after completion).
// ctx cancellation aborts the remote command.
// Output contract matches sshexec.Executor: stdout/stderr per line, non-zero exit emits exit line;
// additionally, on run failure (connect/auth) emits {Stream:"error"} line.
func (e *Executor) Exec(ctx context.Context, command string) (<-chan sshexec.Line, error) {
	client, err := e.newClient(e.host)
	if err != nil {
		return nil, err
	}
	out := make(chan sshexec.Line, 32)
	go func() {
		defer close(out)
		stdout, stderr, exitCode, runErr := client.RunPSWithContext(ctx, command)

		emit := func(stream, text string) {
			for _, ln := range splitLines(text) {
				select {
				case out <- sshexec.Line{Stream: stream, Text: ln}:
				case <-ctx.Done():
					return
				}
			}
		}
		emit("stdout", stdout)
		emit("stderr", stderr)

		if runErr != nil {
			select {
			case out <- sshexec.Line{Stream: "error", Text: runErr.Error()}:
			case <-ctx.Done():
			}
			return
		}
		if exitCode != 0 {
			select {
			case out <- sshexec.Line{Stream: "exit", Text: fmt.Sprintf("exit_code=%d", exitCode)}:
			case <-ctx.Done():
			}
		}
	}()
	return out, nil
}

// defaultNewClient constructs a masterzen/winrm client (NTLM, skip cert verify).
func defaultNewClient(h Host) (runner, error) {
	endpoint := winrm.NewEndpoint(h.Addr, h.Port, httpsForPort(h.Port), true, nil, nil, nil, 0)
	return winrm.NewClient(endpoint, h.User, h.Secret)
}

// httpsForPort infers transport from port: 5986 uses HTTPS, otherwise HTTP.
// Note: transport is inferred from the port number, so custom HTTPS ports are not supported.
func httpsForPort(port int) bool {
	return port == 5986
}

// splitLines splits an output string into lines: strips \r\n, trims trailing empty lines, preserves interior empty lines.
func splitLines(s string) []string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	lines := strings.Split(s, "\n")
	for len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	return lines
}
