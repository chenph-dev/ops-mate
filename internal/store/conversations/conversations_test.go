package convstore

import (
	"testing"

	"ops-mate/internal/store"
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
	if len(conv) != 1 || conv[0].Title != "cpu 高" {
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

func closeDB(app *store.DB) {
	sqlDB, _ := app.GORM().DB()
	if sqlDB != nil {
		sqlDB.Close()
	}
}
