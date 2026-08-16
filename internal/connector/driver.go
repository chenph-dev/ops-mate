package connector

// Config 构造连接所需的配置：通用字段 + driver 专属参数。
type Config struct {
	Addr     string
	Port     int
	User     string
	Password string
	Params   map[string]any // driver 专属参数，key 见 Driver.Params schema
}

// SkillPack 协议 skill 包：统一 agent 按连接类型切换的三件套。
type SkillPack struct {
	Prompt    string // 注入系统提示词的协议片段（DBA / Redis / K8s 语义）
	Guardrail string // guardrail 协议名（sql / linux / windows / redis）
}

// Driver 一种连接类型的完整声明。
type Driver struct {
	Protocol     string        // 注册表 key，如 "mysql"
	Name         string        // 展示名，如 "MySQL"
	NeedsHost    bool          // false 时前端隐藏 host/port/user 区块（如 sqlite 本地文件）
	Params       []ParamSchema // 资产录入表单的专属参数
	Capabilities []string      // 能力标记，如 ["query","objectTree"]
	SkillPack    SkillPack
	New          func(cfg Config) (Capability, error) // 构造能力对象（懒建连）
}

// Capability 连接提供的能力对象，具体实现 QueryRunner / ObjectBrowser / CommandRunner 之一或多个。
type Capability interface{}
