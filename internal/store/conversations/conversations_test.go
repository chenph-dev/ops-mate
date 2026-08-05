package convstore

import (
	"strings"
	"testing"
	"time"

	"ops-mate/internal/store"
	"ops-mate/internal/store/crypto"
	hoststore "ops-mate/internal/store/hosts"
)

func TestConversationAndCommands_FTS(t *testing.T) {
	t.Setenv("APPDATA", t.TempDir())
	app, _ := store.Open()
	defer closeDB(app)

	hosts := hoststore.NewHostsStore(app)
	convs := NewConvStore(app)

	hostID, _ := hosts.SaveHost(hoststore.HostInput{Name: "h", Addr: "1.1.1.1", Port: 22, User: "u", AuthType: "password", Secret: "x"})

	sid, err := convs.NewConversation(hostID, "cpu 高")
	if err != nil {
		t.Fatalf("NewConversation: %v", err)
	}
	if err := convs.SaveMessage(Message{SessionID: sid, Role: "user", Content: "cpu 为什么高"}); err != nil {
		t.Fatalf("SaveMessage: %v", err)
	}
	if err := convs.SaveMessage(Message{SessionID: sid, Role: "assistant", Content: "我看看"}); err != nil {
		t.Fatalf("SaveMessage assistant: %v", err)
	}
	if err := convs.SaveCommand(sid, "top -bn1", 0, "go proc 99%"); err != nil {
		t.Fatalf("SaveCommand: %v", err)
	}

	conv, _ := convs.ListConversations(hostID)
	// 首条 user 消息会把标题更新为内容摘要
	if len(conv) != 1 || conv[0].Title != "cpu 为什么高" {
		t.Fatalf("ListConversations: %+v", conv)
	}
	msgs, _ := convs.LoadMessages(sid)
	if len(msgs) != 2 || msgs[0].Role != "user" {
		t.Fatalf("LoadMessages: %+v", msgs)
	}
}

func TestSaveMessage_ToolFieldsRoundTrip(t *testing.T) {
	t.Setenv("APPDATA", t.TempDir())
	app, _ := store.Open()
	defer closeDB(app)

	hosts := hoststore.NewHostsStore(app)
	convs := NewConvStore(app)
	hostID, _ := hosts.SaveHost(hoststore.HostInput{Name: "h", Addr: "1.1.1.1", Port: 22, User: "u", AuthType: "password", Secret: "x"})
	sid, _ := convs.NewConversation(hostID, "t")

	toolCallsJSON := `[{"id":"call_1","name":"execute_command","arguments":"{\"command\":\"ls\"}"}]`
	if err := convs.SaveMessage(Message{
		SessionID: sid, Role: "assistant", Content: "", ToolCalls: toolCallsJSON,
	}); err != nil {
		t.Fatalf("SaveMessage assistant tool_calls: %v", err)
	}
	if err := convs.SaveMessage(Message{
		SessionID: sid, Role: "tool", Content: "file1\nfile2",
		ToolCallID: "call_1", ToolName: "execute_command",
	}); err != nil {
		t.Fatalf("SaveMessage tool: %v", err)
	}

	msgs, err := convs.LoadMessages(sid)
	if err != nil {
		t.Fatalf("LoadMessages: %v", err)
	}
	if len(msgs) != 2 {
		t.Fatalf("期望 2 条消息，得到 %d", len(msgs))
	}
	if msgs[0].ToolCalls != toolCallsJSON {
		t.Errorf("tool_calls 往返失败: %q", msgs[0].ToolCalls)
	}
	if msgs[1].ToolCallID != "call_1" || msgs[1].ToolName != "execute_command" {
		t.Errorf("tool 消息字段往返失败: %+v", msgs[1])
	}
}

func TestGetConversation(t *testing.T) {
	t.Setenv("APPDATA", t.TempDir())
	app, _ := store.Open()
	defer closeDB(app)

	hosts := hoststore.NewHostsStore(app)
	convs := NewConvStore(app)
	hostID, _ := hosts.SaveHost(hoststore.HostInput{Name: "h", Addr: "1.1.1.1", Port: 22, User: "u", AuthType: "password", Secret: "x"})
	sid, _ := convs.NewConversation(hostID, "标题")

	conv, err := convs.GetConversation(sid)
	if err != nil {
		t.Fatalf("GetConversation: %v", err)
	}
	if conv.HostID != hostID || conv.Title != "标题" {
		t.Errorf("GetConversation 返回错误: %+v", conv)
	}
	if _, err := convs.GetConversation("不存在"); err == nil {
		t.Error("期望不存在的会话返回错误")
	}
}

func TestSaveMessage_ApprovalStatusRoundTrip(t *testing.T) {
	t.Setenv("APPDATA", t.TempDir())
	app, _ := store.Open()
	defer closeDB(app)

	convs := NewConvStore(app)
	hosts := hoststore.NewHostsStore(app)
	hostID, _ := hosts.SaveHost(hoststore.HostInput{Name: "h", Addr: "1.1.1.1", Port: 22, User: "u", AuthType: "password", Secret: "x"})
	sid, _ := convs.NewConversation(hostID, "t")

	if err := convs.SaveMessage(Message{
		SessionID: sid, Role: "tool", Content: "out",
		ToolCallID: "c1", ToolName: "execute_command", ApprovalStatus: "rejected",
	}); err != nil {
		t.Fatalf("SaveMessage: %v", err)
	}
	msgs, err := convs.LoadMessages(sid)
	if err != nil {
		t.Fatalf("LoadMessages: %v", err)
	}
	if len(msgs) != 1 || msgs[0].ApprovalStatus != "rejected" {
		t.Errorf("approval_status 往返失败: %+v", msgs)
	}
}

func TestSaveMessage_FirstUserMessageSetsTitleSummary(t *testing.T) {
	t.Setenv("APPDATA", t.TempDir())
	app, _ := store.Open()
	defer closeDB(app)

	convs := NewConvStore(app)
	hosts := hoststore.NewHostsStore(app)
	hostID, _ := hosts.SaveHost(hoststore.HostInput{Name: "h", Addr: "1.1.1.1", Port: 22, User: "u", AuthType: "password", Secret: "x"})
	sid, _ := convs.NewConversation(hostID, "对话 初始")

	// 首条 user 消息应把标题更新为 20 字符摘要
	longText := strings.Repeat("测", 30)
	if err := convs.SaveMessage(Message{SessionID: sid, Role: "user", Content: longText}); err != nil {
		t.Fatalf("SaveMessage: %v", err)
	}
	conv, _ := convs.GetConversation(sid)
	want := strings.Repeat("测", 20) + "..."
	if conv.Title != want {
		t.Errorf("标题应为摘要 %q，得到 %q", want, conv.Title)
	}

	// 第二条消息不应覆盖标题
	if err := convs.SaveMessage(Message{SessionID: sid, Role: "user", Content: "第二条"}); err != nil {
		t.Fatalf("SaveMessage 2: %v", err)
	}
	conv, _ = convs.GetConversation(sid)
	if conv.Title != want {
		t.Errorf("第二条消息不应覆盖标题: %q", conv.Title)
	}
}

func TestLoadMessages_SameTsOrderByInsert(t *testing.T) {
	t.Setenv("APPDATA", t.TempDir())
	app, _ := store.Open()
	defer closeDB(app)
	convs := NewConvStore(app)
	hosts := hoststore.NewHostsStore(app)
	hostID, _ := hosts.SaveHost(hoststore.HostInput{Name: "h", Addr: "1.1.1.1", Port: 22, User: "u", AuthType: "password", Secret: "x"})
	sid, err := convs.NewConversation(hostID, "t")
	if err != nil {
		t.Fatalf("NewConversation: %v", err)
	}

	// 同秒落库多轮审批消息：user / assistant(tool_calls) / tool / assistant(final)。
	// LoadMessages 必须按插入顺序返回，否则前端"assistant 命令卡后面紧跟 tool 结果"
	// 的审批状态推断会落到 pending。
	now := time.Now().Unix()
	rows := []convMessage{
		{ID: crypto.NewID(), SessionID: sid, Role: "user", Content: "查一下", Ts: now},
		{ID: crypto.NewID(), SessionID: sid, Role: "assistant", Content: "", ToolCalls: strPtr(`[{"id":"c1","name":"execute_command","arguments":"{\"command\":\"ls\"}"}]`), Ts: now},
		{ID: crypto.NewID(), SessionID: sid, Role: "tool", Content: "file1", ToolCallID: strPtr("c1"), ToolName: strPtr("execute_command"), Ts: now},
		{ID: crypto.NewID(), SessionID: sid, Role: "assistant", Content: "完成", Ts: now},
	}
	for _, r := range rows {
		if err := app.GORM().Create(&r).Error; err != nil {
			t.Fatalf("insert: %v", err)
		}
	}

	msgs, err := convs.LoadMessages(sid)
	if err != nil {
		t.Fatalf("LoadMessages: %v", err)
	}
	if len(msgs) != 4 {
		t.Fatalf("期望 4 条，得到 %d: %+v", len(msgs), msgs)
	}
	if msgs[1].Role != "assistant" || msgs[1].ToolCalls == "" {
		t.Errorf("第 2 条应为 assistant(tool_calls)，得到: role=%s toolCalls=%q", msgs[1].Role, msgs[1].ToolCalls)
	}
	if msgs[2].Role != "tool" || msgs[2].ToolCallID != "c1" {
		t.Errorf("第 3 条应为配对 tool(c1)，得到: %+v", msgs[2])
	}
}

func closeDB(app *store.DB) {
	sqlDB, _ := app.GORM().DB()
	if sqlDB != nil {
		sqlDB.Close()
	}
}
