package connector

// Config 构造连接所需的配置：通用字段 + connector 专属参数。
type Config struct {
	Addr     string
	Port     int
	User     string
	Password string
	Params   map[string]any // connector 专属参数，key 见 Connector.Params schema
}

// Guardrail 危险判定协议（映射 guardrail 包的协议语义）。
// 注意 Windows 语义在 guardrail 包的协议名是 "winrm"（历史命名），非 "windows"。
type Guardrail string

const (
	GuardrailSQL     Guardrail = "sql"   // SQL 语义（只读查询 auto、高危 DDL approve）
	GuardrailLinux   Guardrail = "linux" // Linux shell 语义（guardrail 包默认分支）
	GuardrailWindows Guardrail = "winrm" // Windows PowerShell 语义
	GuardrailRedis   Guardrail = "redis" // Redis 语义（guardrail 包当前回落 Linux，供未来驱动）
)

// SkillPack 协议 skill 包：统一 agent 按连接类型切换的三件套。
type SkillPack struct {
	Prompt    string    // 注入系统提示词的协议片段（DBA / Redis / K8s 语义）
	Guardrail Guardrail // guardrail 协议名（sql / linux / winrm / redis）
}

// ConnectorKind 连接类型的消费模型：能力型（数据库查询）还是命令型（shell 执行）。
type ConnectorKind string

const (
	KindDB      ConnectorKind = "db"      // 能力型：New 返回 QueryRunner/Pingable，走 execute_sql / DbPanel
	KindCommand ConnectorKind = "command" // 命令型：走 execute_command / 终端 / SFTP，执行器按 CommandKind 构造
)

// CommandKind 命令型驱动的执行器协议名。
const (
	CommandSSH   = "ssh"
	CommandWinRM = "winrm"
)

// Connector 一种连接类型的完整声明。所有驱动必须显式声明 Kind（无隐式零值语义）。
type Connector struct {
	Protocol    string          // 注册表 key，如 "mysql"
	Name        string          // 展示名，如 "MySQL"
	Kind        ConnectorKind   // 必须显式声明（db / command），Register 校验
	CommandKind string          // Kind==command 时：执行器协议名（CommandSSH / CommandWinRM）
	NeedsHost   bool            // false 时前端隐藏 host/port/user 区块（如 sqlite 本地文件）
	Params      []ParamSchema   // 资产录入表单的专属参数
	SkillPack   SkillPack
	New         func(cfg Config) (Capability, error) // 构造能力对象（懒建连），DB 型使用
}

// IsDB 是否为能力型（数据库）驱动。
func (c *Connector) IsDB() bool { return c != nil && c.Kind == KindDB }

// IsCommand 是否为命令型（shell 执行）驱动。
func (c *Connector) IsCommand() bool { return c != nil && c.Kind == KindCommand }

// Capability 连接提供的能力对象，具体实现 QueryRunner / Pingable 之一或多个。
type Capability interface{}
