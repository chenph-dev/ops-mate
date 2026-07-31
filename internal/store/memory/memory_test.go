package memorystore

import (
	"testing"

	"ops-mate/internal/store"
	convstore "ops-mate/internal/store/conversations"
	hoststore "ops-mate/internal/store/hosts"
)

func TestMemory_RecallReturnsPastCommands(t *testing.T) {
	t.Setenv("APPDATA", t.TempDir())
	app, _ := store.Open()
	defer closeDB(app)

	hosts := hoststore.NewHostsStore(app)
	convs := convstore.NewConvStore(app)
	mem := NewMemoryStore(app)

	hostID, _ := hosts.SaveHost(hoststore.HostInput{Name: "h", Addr: "1.1.1.1", Port: 22, User: "u", AuthType: "password", Secret: "x"})
	sid, _ := convs.NewConversation(hostID, "old")
	convs.SaveCommand(sid, "top -bn1", 0, "go proc 占满 CPU")
	convs.SaveCommand(sid, "journalctl -u nginx", 0, "nginx restarted")

	ctx, err := mem.Recall(hostID, "CPU 高怎么回事")
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

func closeDB(app *store.DB) {
	sqlDB, _ := app.GORM().DB()
	if sqlDB != nil {
		sqlDB.Close()
	}
}
