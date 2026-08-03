package sshexec

import (
	"context"
	"io"
	"sync"
	"time"

	"golang.org/x/crypto/ssh"
)

// keepaliveInterval 心跳间隔：周期性向远端发送 keepalive 请求，检测静默断连。
const keepaliveInterval = 30 * time.Second

// Session 一个持久化的交互式 SSH 会话（PTY + shell），用于真实终端。
// 区别于 Executor（单命令、按行输出），Session 支持双向原始字节流。
type Session struct {
	client *ssh.Client
	shell  *ssh.Session
	stdin  io.WriteCloser
	stdout io.Reader
	stderr io.Reader
	mu     sync.Mutex
	closed bool
}

// OpenSession 建立交互式 SSH 会话并启动远程 shell（PTY）。
// cols/rows 为终端初始尺寸。注意：stdin/stdout/stderr 管道必须在 Shell() 之前配置。
func OpenSession(ctx context.Context, host Host, cols, rows int) (*Session, error) {
	client, err := dial(ctx, host)
	if err != nil {
		return nil, err
	}
	s, err := client.NewSession()
	if err != nil {
		client.Close()
		return nil, err
	}
	// 请求 PTY，使远程 shell 进入交互模式（vim/top 等依赖）。
	if err := s.RequestPty("xterm-256color", rows, cols, ssh.TerminalModes{
		ssh.ECHO:          1,
		ssh.TTY_OP_ISPEED: 14400,
		ssh.TTY_OP_OSPEED: 14400,
	}); err != nil {
		s.Close()
		client.Close()
		return nil, err
	}
	stdin, err := s.StdinPipe()
	if err != nil {
		s.Close()
		client.Close()
		return nil, err
	}
	stdout, err := s.StdoutPipe()
	if err != nil {
		s.Close()
		client.Close()
		return nil, err
	}
	stderr, err := s.StderrPipe()
	if err != nil {
		s.Close()
		client.Close()
		return nil, err
	}
	if err := s.Shell(); err != nil {
		s.Close()
		client.Close()
		return nil, err
	}
	sess := &Session{client: client, shell: s, stdin: stdin, stdout: stdout, stderr: stderr}
	go sess.keepalive()
	return sess, nil
}

// keepalive 周期性发送 SSH keepalive 请求，检测连接是否仍存活。
// 连接失效（网络中断/服务端超时）时关闭会话，触发输出流结束 → terminal:closed 事件。
func (s *Session) keepalive() {
	ticker := time.NewTicker(keepaliveInterval)
	defer ticker.Stop()
	for range ticker.C {
		s.mu.Lock()
		closed := s.closed
		s.mu.Unlock()
		if closed {
			return
		}
		// 发送 keepalive 全局请求；连接失效时返回错误。
		if _, _, err := s.client.SendRequest("keepalive@openssh.com", true, nil); err != nil {
			s.Close()
			return
		}
	}
}

// Output 读取远程 shell 的原始输出（含 ANSI 转义序列），返回一个流。
// 调用方负责消费完后再调用 Close；流关闭表示会话结束。
func (s *Session) Output() <-chan []byte {
	out := make(chan []byte, 64)
	go func() {
		defer close(out)
		pipe := func(r io.Reader) {
			buf := make([]byte, 4096)
			for {
				n, err := r.Read(buf)
				if n > 0 {
					chunk := make([]byte, n)
					copy(chunk, buf[:n])
					select {
					case out <- chunk:
					default:
						// 消费者慢时丢弃，避免阻塞。
					}
				}
				if err != nil {
					return
				}
			}
		}
		go pipe(s.stdout)
		pipe(s.stderr)
		s.shell.Wait()
	}()
	return out
}

// Input 将原始字节写入远程 shell 的 stdin。
func (s *Session) Input(p []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return io.EOF
	}
	_, err := s.stdin.Write(p)
	return err
}

// Resize 更新远程 PTY 尺寸（cols 列、rows 行）。
func (s *Session) Resize(cols, rows int) error {
	return s.shell.WindowChange(rows, cols)
}

// Close 关闭 ssh 会话与连接。
func (s *Session) Close() {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return
	}
	s.closed = true
	s.mu.Unlock()
	s.shell.Close()
	s.client.Close()
}