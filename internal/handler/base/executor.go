package base

import (
	"fmt"
	"log"
	"strings"

	"ops-mate/internal/connector"
	"ops-mate/internal/dbexec"
	"ops-mate/internal/sshexec"
	hoststore "ops-mate/internal/store/hosts"
	"ops-mate/internal/winrmexec"
)

// ExecutorForHost 按协议构造执行器。protocol 为空或非 winrm 视作 ssh。
// 接收裸字段（保存前的 HostInput 或已解析的 meta），供 TestConnection 等
// 尚未落库的场景使用。jdbc 协议不实现 sshexec.Exec（结构化结果），返回 nil，
// 请使用 ExecutorResolver.DbFor 或 dbexec 直接构造。
func ExecutorForHost(protocol string, addr string, port int, user, authType, secret string) sshexec.Exec {
	if strings.EqualFold(protocol, "winrm") {
		return winrmexec.NewExecutor(winrmexec.Host{Addr: addr, Port: port, User: user, Secret: secret})
	}
	if strings.EqualFold(protocol, "jdbc") {
		return nil
	}
	return sshexec.NewExecutor(sshexec.Host{Addr: addr, Port: port, User: user, AuthType: authType, Secret: secret})
}

// ExecutorResolver 按 hostID 解析连接对象，内部完成凭据读取 + 元信息读取 +
// 协议分流 + Host 拼装。收敛原先散落在 main.go / terminal.go 的重复转换，
// 调用方只依赖协议无关的 sshexec.Exec / *sshexec.Host，不再感知协议存在。
type ExecutorResolver struct {
	hosts *hoststore.HostsStore
}

// NewExecutorResolver 构造 ExecutorResolver。hosts 可为 nil（此时 ExecFor/HostFor 返回失败）。
func NewExecutorResolver(hosts *hoststore.HostsStore) *ExecutorResolver {
	return &ExecutorResolver{hosts: hosts}
}

// ExecFor 按 hostID 构造协议对应的执行器（AI 会话、命令执行、连接测试、命令补全共用）。
// 凭据或元信息解析失败返回 nil（与 session 层 executorFor 契约一致：nil = 不可用）。
func (r *ExecutorResolver) ExecFor(hostID string) sshexec.Exec {
	if r == nil || r.hosts == nil {
		return nil
	}
	secret, authType, err := r.hosts.GetHostSecret(hostID)
	if err != nil {
		return nil
	}
	meta, err := r.hosts.HostMetaByID(hostID)
	if err != nil || meta == nil {
		return nil
	}
	return ExecutorForHost(meta.Protocol, meta.Addr, meta.Port, meta.User, authType, secret)
}

// HostFor 按 hostID 构造 SSH 目标（交互式终端 / SFTP 用）。
// WinRM 资产返回错误——交互式会话与 SFTP 仅支持 SSH 协议。
func (r *ExecutorResolver) HostFor(hostID string) (*sshexec.Host, error) {
	if r == nil || r.hosts == nil {
		return nil, fmt.Errorf("资产解析器未配置")
	}
	meta, err := r.hosts.HostMetaByID(hostID)
	if err != nil {
		return nil, fmt.Errorf("获取资产信息失败: %w", err)
	}
	if meta == nil {
		return nil, fmt.Errorf("资产不存在")
	}
	if strings.EqualFold(meta.Protocol, "winrm") {
		return nil, fmt.Errorf("WinRM 资产不支持交互式会话")
	}
	if strings.EqualFold(meta.Protocol, "jdbc") {
		return nil, fmt.Errorf("数据库资产不支持交互式会话")
	}
	if connector.Get(meta.Protocol) != nil {
		return nil, fmt.Errorf("数据库资产不支持交互式会话")
	}
	secret, authType, err := r.hosts.GetHostSecret(hostID)
	if err != nil {
		return nil, fmt.Errorf("获取资产凭据失败: %w", err)
	}
	return &sshexec.Host{
		Addr: meta.Addr, Port: meta.Port, User: meta.User,
		AuthType: authType, Secret: secret,
	}, nil
}

// DbFor 按 hostID 构造数据库执行器（数据库协议，db 工作台用）。
// 未注册协议、凭据或元信息解析失败返回 nil。
// 注意：当前注册的数据库驱动（mysql/postgres/sqlite）均为 dbexec 后端，
// protocol 即 dbexec 驱动名；若未来注册非 dbexec 的连接类型（Redis/K8s 等），
// 此处需改为按 Driver.DBBacked 标记或独立路由，避免构造出无效执行器。
func (r *ExecutorResolver) DbFor(hostID string) *dbexec.Executor {
	if r == nil || r.hosts == nil {
		return nil
	}
	secret, _, err := r.hosts.GetHostSecret(hostID)
	if err != nil {
		return nil
	}
	meta, err := r.hosts.HostMetaByID(hostID)
	if err != nil || meta == nil {
		return nil
	}
	if connector.Get(meta.Protocol) == nil {
		return nil
	}
	return dbexec.NewExecutor(dbexec.Host{
		Driver: meta.Protocol, Addr: meta.Addr, Port: meta.Port,
		User: meta.User, Password: secret, Database: paramsDatabase(meta.Params),
	})
}

// ConnFor 按 hostID 构造连接能力对象（protocol 对应 Driver.New 的产物）。
// 已注册连接类型（数据库等）返回其能力（QueryRunner/ObjectBrowser/Pingable）；
// ssh/winrm 等未注册类型返回 nil（AI 命令工具走 holder，交互式会话走 HostFor）。
func (r *ExecutorResolver) ConnFor(hostID string) connector.Capability {
	if r == nil || r.hosts == nil {
		return nil
	}
	secret, _, err := r.hosts.GetHostSecret(hostID)
	if err != nil {
		log.Printf("resolver: connfor host %s: %v", hostID, err)
		return nil
	}
	meta, err := r.hosts.HostMetaByID(hostID)
	if err != nil {
		log.Printf("resolver: connfor host %s: %v", hostID, err)
		return nil
	}
	if meta == nil {
		log.Printf("resolver: connfor host %s: meta not found", hostID)
		return nil
	}
	if connector.Get(meta.Protocol) == nil {
		return nil
	}
	cap, err := connector.New(meta.Protocol, connector.Config{
		Addr: meta.Addr, Port: meta.Port, User: meta.User,
		Password: secret, Params: meta.Params,
	})
	if err != nil {
		log.Printf("resolver: connfor host %s: %v", hostID, err)
		return nil
	}
	return cap
}

// paramsDatabase 从资产专属参数取数据库名（mysql/postgres 的 database，sqlite 的 filePath）。
func paramsDatabase(params map[string]any) string {
	if v, ok := params["database"].(string); ok {
		return v
	}
	if v, ok := params["filePath"].(string); ok {
		return v
	}
	return ""
}
