package handler

import (
	"strings"

	"ops-mate/internal/sshexec"
	"ops-mate/internal/winrmexec"
)

// executorForHost 按协议构造执行器。protocol 为空或非 winrm 视作 ssh。
func executorForHost(protocol string, addr string, port int, user, authType, secret string) sshexec.Exec {
	if strings.EqualFold(protocol, "winrm") {
		return winrmexec.NewExecutor(winrmexec.Host{Addr: addr, Port: port, User: user, Secret: secret})
	}
	return sshexec.NewExecutor(sshexec.Host{Addr: addr, Port: port, User: user, AuthType: authType, Secret: secret})
}
