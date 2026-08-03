package sshexec

import (
	"bufio"
	"context"
	"fmt"
	"io"

	"golang.org/x/crypto/ssh"
)

// Host 描述一个 SSH 目标。Secret 为密码或私钥 PEM 明文。
type Host struct {
	Addr     string // host 或 host:port
	Port     int
	User     string
	AuthType string // "password" | "privatekey"
	Secret   string
}

// Line 一行输出，带来源 stdout/stderr。
type Line struct {
	Stream string `json:"stream"` // "stdout" | "stderr"
	Text   string `json:"text"`
}

// Exec 执行器接口（供测试 stub 与真实 Executor 实现）。
// 执行器构造时已绑定目标主机，故 Exec 只接收命令本身。
type Exec interface {
	Exec(ctx context.Context, command string) (<-chan Line, error)
}

// Executor 执行命令并逐行流式输出。
type Executor struct{ host Host }

func NewExecutor(h Host) *Executor { return &Executor{host: h} }

func (e *Executor) dial(ctx context.Context) (*ssh.Client, error) {
	return dial(ctx, e.host)
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
		if err := sess.Wait(); err != nil {
			if exitErr, ok := err.(*ssh.ExitError); ok {
				select {
				case out <- Line{Stream: "exit", Text: fmt.Sprintf("exit_code=%d", exitErr.ExitStatus())}:
				default:
				}
			}
		}
		close(out)
	}()
	return out, nil
}
