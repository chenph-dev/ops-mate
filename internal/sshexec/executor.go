package sshexec

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net"
	"strconv"
	"strings"
	"time"

	"golang.org/x/crypto/ssh"
)

// Host 描述一个 SSH 目标。Secret 为密码或私钥 PEM 明文。
type Host struct {
	Addr      string // host 或 host:port
	Port      int
	User      string
	AuthType  string // "password" | "privatekey"
	Secret    string
}

// Line 一行输出，带来源 stdout/stderr。
type Line struct {
	Stream string `json:"stream"` // "stdout" | "stderr"
	Text   string `json:"text"`
}

// Executor 执行命令并逐行流式输出。
type Executor struct{ host Host }

func NewExecutor(h Host) *Executor { return &Executor{host: h} }

func (e *Executor) dial(ctx context.Context) (*ssh.Client, error) {
	addr := e.host.Addr
	if !strings.Contains(addr, ":") {
		port := 22
		if e.host.Port != 0 {
			port = e.host.Port
		}
		addr = net.JoinHostPort(addr, strconv.Itoa(port))
	}
	auth, err := e.authMethod()
	if err != nil {
		return nil, err
	}
	cfg := &ssh.ClientConfig{
		User:            e.host.User,
		Auth:            []ssh.AuthMethod{auth},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         10 * time.Second,
	}
	d := net.Dialer{}
	conn, err := d.DialContext(ctx, "tcp", addr)
	if err != nil {
		return nil, fmt.Errorf("dial %s: %w", addr, err)
	}
	c, chans, reqs, err := ssh.NewClientConn(conn, addr, cfg)
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("ssh handshake: %w", err)
	}
	return ssh.NewClient(c, chans, reqs), nil
}

func (e *Executor) authMethod() (ssh.AuthMethod, error) {
	switch e.host.AuthType {
	case "password":
		return ssh.Password(e.host.Secret), nil
	case "privatekey":
		signer, err := ssh.ParsePrivateKey([]byte(e.host.Secret))
		if err != nil {
			return nil, fmt.Errorf("parse private key: %w", err)
		}
		return ssh.PublicKeys(signer), nil
	default:
		return nil, fmt.Errorf("未知 auth_type %q", e.host.AuthType)
	}
}

// Exec 执行命令，返回行流通道（执行结束后关闭）。ctx 取消则中止会话。
func (e *Executor) Exec(ctx context.Context, command string) (<-chan Line, error) {
	client, err := e.dial(ctx)
	if err != nil {
		return nil, err
	}
	sess, err := client.NewSession()
	if err != nil {
		client.Close()
		return nil, err
	}
	stdout, err := sess.StdoutPipe()
	if err != nil {
		client.Close()
		return nil, err
	}
	stderr, err := sess.StderrPipe()
	if err != nil {
		client.Close()
		return nil, err
	}
	if err := sess.Start(command); err != nil {
		client.Close()
		return nil, fmt.Errorf("start: %w", err)
	}
	out := make(chan Line, 32)
	go func() {
		defer client.Close()
		pipe := func(r io.Reader, stream string) {
			sc := bufio.NewScanner(r)
			for sc.Scan() {
				select {
				case out <- Line{Stream: stream, Text: sc.Text()}:
				case <-ctx.Done():
					return
				}
			}
		}
		go pipe(stdout, "stdout")
		pipe(stderr, "stderr")
		_ = sess.Wait()
		close(out)
	}()
	return out, nil
}
