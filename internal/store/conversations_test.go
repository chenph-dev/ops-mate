package store

import "testing"

func TestConversationAndCommands_FTS(t *testing.T) {
	t.Setenv("APPDATA", t.TempDir())
	s, _ := Open()
	defer s.DB.Close()

	hostID, _ := s.SaveHost(HostInput{Name: "h", Addr: "1.1.1.1", Port: 22, User: "u", AuthType: "password", Secret: "x"})

	sid, err := s.NewConversation(hostID, "cpu 高")
	if err != nil {
		t.Fatalf("NewConversation: %v", err)
	}
	if err := s.AppendMessage(sid, "user", "cpu 为什么高", ""); err != nil {
		t.Fatalf("AppendMessage: %v", err)
	}
	if err := s.AppendMessage(sid, "assistant", "我看看", ""); err != nil {
		t.Fatalf("AppendMessage assistant: %v", err)
	}
	if err := s.SaveCommand(sid, "top -bn1", 0, "go proc 99%"); err != nil {
		t.Fatalf("SaveCommand: %v", err)
	}

	conv, _ := s.ListConversations(hostID)
	if len(conv) != 1 || conv[0].Title != "cpu 高" {
		t.Fatalf("ListConversations: %+v", conv)
	}
	msgs, _ := s.LoadMessages(sid)
	if len(msgs) != 2 || msgs[0].Role != "user" {
		t.Fatalf("LoadMessages: %+v", msgs)
	}
}
