package convstore

import (
	"testing"

	"ops-mate/internal/store"
	hoststore "ops-mate/internal/store/hosts"
)

func TestConversationAndCommands_FTS(t *testing.T) {
	t.Setenv("APPDATA", t.TempDir())
	app, _ := store.Open()
	defer app.DB().Close()

	hosts := hoststore.NewHostsStore(app)
	convs := NewConvStore(app)

	hostID, _ := hosts.SaveHost(hoststore.HostInput{Name: "h", Addr: "1.1.1.1", Port: 22, User: "u", AuthType: "password", Secret: "x"})

	sid, err := convs.NewConversation(hostID, "cpu 高")
	if err != nil {
		t.Fatalf("NewConversation: %v", err)
	}
	if err := convs.AppendMessage(sid, "user", "cpu 为什么高", ""); err != nil {
		t.Fatalf("AppendMessage: %v", err)
	}
	if err := convs.AppendMessage(sid, "assistant", "我看看", ""); err != nil {
		t.Fatalf("AppendMessage assistant: %v", err)
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
