package hoststore

import (
	"testing"

	"ops-mate/internal/store"
)

func TestHostCRUD_AuthEncrypted(t *testing.T) {
	t.Setenv("APPDATA", t.TempDir())
	app, err := store.Open()
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer app.DB().Close()

	s := NewHostsStore(app)
	h := HostInput{
		Name: "web-01", Addr: "10.0.0.5", Port: 22, User: "ops",
		AuthType: "password", Secret: "p@ss",
	}
	id, err := s.SaveHost(h)
	if err != nil {
		t.Fatalf("SaveHost: %v", err)
	}

	var blob []byte
	if err := app.DB().QueryRow(`SELECT auth_encrypted FROM hosts WHERE id=?`, id).Scan(&blob); err != nil {
		t.Fatalf("查 auth_encrypted: %v", err)
	}
	if string(blob) == "p@ss" {
		t.Fatal("密码被明文存储")
	}

	list, err := s.ListHosts()
	if err != nil {
		t.Fatalf("ListHosts: %v", err)
	}
	if len(list) != 1 || list[0].Name != "web-01" {
		t.Fatalf("ListHosts 结果: %+v", list)
	}

	sec, at, err := s.GetHostSecret(id)
	if err != nil {
		t.Fatalf("GetHostSecret: %v", err)
	}
	if sec != "p@ss" || at != "password" {
		t.Fatalf("GetHostSecret = %q %q", sec, at)
	}

	if err := s.DeleteHost(id); err != nil {
		t.Fatalf("DeleteHost: %v", err)
	}
	list, _ = s.ListHosts()
	if len(list) != 0 {
		t.Fatal("删除后应无主机")
	}
}
