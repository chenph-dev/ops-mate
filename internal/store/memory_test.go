package store

import "testing"

func TestMemory_RecallReturnsPastCommands(t *testing.T) {
	t.Setenv("APPDATA", t.TempDir())
	s, _ := Open()
	defer s.DB.Close()

	hostID, _ := s.SaveHost(HostInput{Name: "h", Addr: "1.1.1.1", Port: 22, User: "u", AuthType: "password", Secret: "x"})
	sid, _ := s.NewConversation(hostID, "old")
	s.SaveCommand(sid, "top -bn1", 0, "go proc 占满 CPU")
	s.SaveCommand(sid, "journalctl -u nginx", 0, "nginx restarted")

	ctx, err := s.Recall(hostID, "CPU 高怎么回事")
	if err != nil {
		t.Fatalf("Recall: %v", err)
	}
	if len(ctx.PastCommands) == 0 {
		t.Fatal("应召回过往命令")
	}
	hit := false
	for _, c := range ctx.PastCommands {
		if c.Command == "top -bn1" {
			hit = true
		}
	}
	if !hit {
		t.Fatalf("应召回 top 命令，得到 %+v", ctx.PastCommands)
	}
}
