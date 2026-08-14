package sshexec

import (
	"context"
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"

	"golang.org/x/crypto/ssh"
)

// Dial 建立到目标资产的 SSH 连接，供 Executor（单命令）、Session（交互终端）与 SFTP 复用。
func Dial(ctx context.Context, host Host) (*ssh.Client, error) {
	addr := host.Addr
	if !strings.Contains(addr, ":") {
		port := 22
		if host.Port != 0 {
			port = host.Port
		}
		addr = net.JoinHostPort(addr, strconv.Itoa(port))
	}
	auth, err := authMethod(host)
	if err != nil {
		return nil, err
	}
	cfg := &ssh.ClientConfig{
		User:            host.User,
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

// authMethod 根据 host.AuthType 构造认证方式。
func authMethod(host Host) (ssh.AuthMethod, error) {
	switch host.AuthType {
	case "password":
		return ssh.Password(host.Secret), nil
	case "privatekey":
		signer, err := ssh.ParsePrivateKey([]byte(host.Secret))
		if err != nil {
			return nil, fmt.Errorf("parse private key: %w", err)
		}
		return ssh.PublicKeys(signer), nil
	default:
		return nil, fmt.Errorf("未知 auth_type %q", host.AuthType)
	}
}
