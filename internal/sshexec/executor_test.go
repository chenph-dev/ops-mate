package sshexec

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/pem"
	"fmt"
	"net"
	"strings"
	"testing"
	"time"

	"golang.org/x/crypto/ssh"
)

// startTestSSHServer 起一个本地 SSH server，用传入公钥授权。
func startTestSSHServer(t *testing.T) (string, string, func()) {
	t.Helper()
	pubKey, privKey, _ := ed25519.GenerateKey(rand.Reader)
	sshPub, _ := ssh.NewPublicKey(pubKey)

	// 服务器自身 host key（必须，否则无法完成握手）
	_, hostPriv, _ := ed25519.GenerateKey(rand.Reader)
	hostSigner, _ := ssh.NewSignerFromKey(hostPriv)

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	config := &ssh.ServerConfig{}
	config.AddHostKey(hostSigner)
	config.PublicKeyCallback = func(c ssh.ConnMetadata, k ssh.PublicKey) (*ssh.Permissions, error) {
		if bytesEqual(k.Marshal(), sshPub.Marshal()) {
			return nil, nil
		}
		return nil, fmt.Errorf("unknown key")
	}

	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go func(c net.Conn) {
				_, chans, reqs, err := ssh.NewServerConn(c, config)
				if err != nil {
					return
				}
				go ssh.DiscardRequests(reqs)
				for newCh := range chans {
					ch, reqs, _ := newCh.Accept()
					go func(ch ssh.Channel, reqs <-chan *ssh.Request) {
						for r := range reqs {
							if r.Type == "exec" {
								var cmd struct{ Cmd string }
								ssh.Unmarshal(r.Payload, &cmd)
								r.Reply(true, nil)
								fmt.Fprintf(ch, "ran: %s\n", cmd.Cmd)
								ch.SendRequest("exit-status", false, ssh.Marshal(struct{ C uint32 }{0}))
								ch.Close()
							}
						}
					}(ch, reqs)
				}
			}(conn)
		}
	}()

	pemBlock, _ := ssh.MarshalPrivateKey(privKey, "")
	return ln.Addr().String(), string(pem.EncodeToMemory(pemBlock)), func() { ln.Close() }
}

func bytesEqual(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestSSHExecutor_StreamsOutput(t *testing.T) {
	addr, privPEM, cleanup := startTestSSHServer(t)
	defer cleanup()

	host := Host{
		Addr: addr, Port: 0, User: "test",
		AuthType: "privatekey", Secret: privPEM,
	}
	ex := NewExecutor(host)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	lines, err := ex.Exec(ctx, "echo hi")
	if err != nil {
		t.Fatalf("Exec: %v", err)
	}
	var out strings.Builder
	for ln := range lines {
		out.WriteString(ln.Text)
	}
	if !strings.Contains(out.String(), "ran: echo hi") {
		t.Fatalf("期望包含执行输出，得到 %q", out.String())
	}
}
