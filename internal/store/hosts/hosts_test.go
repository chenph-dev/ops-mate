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
	defer closeDB(app)

	s := NewHostsStore(app)
	h := HostInput{
		Name: "web-01", ParentID: "", Addr: "10.0.0.5", Port: 22, User: "ops",
		AuthType: "password", Secret: "p@ss",
	}
	id, err := s.SaveHost(h)
	if err != nil {
		t.Fatalf("SaveHost: %v", err)
	}

	var dbHost Host
	if err := app.GORM().First(&dbHost, "id = ?", id).Error; err != nil {
		t.Fatalf("查 host: %v", err)
	}
	if string(dbHost.AuthEncrypted) == "p@ss" {
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

	if err := s.DeleteNode(id); err != nil {
		t.Fatalf("DeleteNode: %v", err)
	}
	list, _ = s.ListHosts()
	if len(list) != 0 {
		t.Fatal("删除后应无资产")
	}
}

func TestHostTree_FolderAndMove(t *testing.T) {
	t.Setenv("APPDATA", t.TempDir())
	app, err := store.Open()
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer closeDB(app)

	s := NewHostsStore(app)

	// 创建目录
	folderID, err := s.CreateFolder("生产环境", "")
	if err != nil {
		t.Fatalf("CreateFolder: %v", err)
	}

	// 在目录下创建资产
	hostID, err := s.SaveHost(HostInput{
		Name: "web-01", ParentID: folderID, Addr: "10.0.0.5",
		Port: 22, User: "ops", AuthType: "password", Secret: "p@ss",
	})
	if err != nil {
		t.Fatalf("SaveHost: %v", err)
	}

	// 验证树形结构
	tree, err := s.ListTree()
	if err != nil {
		t.Fatalf("ListTree: %v", err)
	}
	if len(tree) != 1 || tree[0].Name != "生产环境" {
		t.Fatalf("根级应有 1 个目录，得到 %+v", tree)
	}
	if len(tree[0].Children) != 1 || tree[0].Children[0].Name != "web-01" {
		t.Fatalf("目录下应有 1 个资产，得到 %+v", tree[0].Children)
	}

	// 创建第二个目录，移动资产过去
	folder2ID, err := s.CreateFolder("测试环境", "")
	if err != nil {
		t.Fatalf("CreateFolder2: %v", err)
	}
	if err := s.MoveNode(hostID, folder2ID); err != nil {
		t.Fatalf("MoveNode: %v", err)
	}

	tree, _ = s.ListTree()
	if len(tree) != 2 {
		t.Fatalf("应有 2 个根级目录，得到 %d", len(tree))
	}

	// 删除目录（级联删除子资产）
	if err := s.DeleteNode(folder2ID); err != nil {
		t.Fatalf("DeleteNode: %v", err)
	}
	list, _ := s.ListHosts()
	if len(list) != 0 {
		t.Fatal("级联删除后应无资产")
	}
}

func closeDB(app *store.DB) {
	sqlDB, _ := app.GORM().DB()
	if sqlDB != nil {
		sqlDB.Close()
	}
}

func TestHostProtocolRdpPort_Roundtrip(t *testing.T) {
	t.Setenv("APPDATA", t.TempDir())
	app, err := store.Open()
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer closeDB(app)

	s := NewHostsStore(app)

	// winrm host: save protocol + rdp_port and read back
	id, err := s.SaveHost(HostInput{
		Name: "win01", Addr: "10.0.0.9", Port: 5985, User: "admin",
		AuthType: "password", Secret: "x", Protocol: "winrm", RdpPort: 3390,
	})
	if err != nil {
		t.Fatalf("SaveHost: %v", err)
	}
	meta, err := s.HostMetaByID(id)
	if err != nil {
		t.Fatalf("HostMetaByID: %v", err)
	}
	if meta.Protocol != "winrm" || meta.RdpPort != 3390 {
		t.Errorf("winrm fields not persisted: %+v", meta)
	}

	// default normalization: empty protocol -> ssh, 0 rdp_port -> 3389
	id2, _ := s.SaveHost(HostInput{
		Name: "linux01", Addr: "10.0.0.10", Port: 22, User: "root",
		AuthType: "password", Secret: "x",
	})
	meta2, _ := s.HostMetaByID(id2)
	if meta2.Protocol != "ssh" || meta2.RdpPort != 3389 {
		t.Errorf("default normalization wrong: %+v", meta2)
	}

	// UpdateHost updates protocol
	if err := s.UpdateHost(id, HostInput{
		Name: "win01", Addr: "10.0.0.9", Port: 5986, User: "admin",
		AuthType: "password", Protocol: "winrm", RdpPort: 3390,
	}); err != nil {
		t.Fatalf("UpdateHost: %v", err)
	}
	meta, _ = s.HostMetaByID(id)
	if meta.Port != 5986 {
		t.Errorf("UpdateHost did not update port: %+v", meta)
	}

	// ListTree fills new fields
	tree, _ := s.ListTree()
	var found bool
	for _, n := range tree {
		if n.ID == id && n.Protocol == "winrm" && n.RdpPort == 3390 {
			found = true
		}
	}
	if !found {
		t.Error("ListTree did not fill protocol/rdpPort")
	}
}

func TestHostAutoApprove_Roundtrip(t *testing.T) {
	t.Setenv("APPDATA", t.TempDir())
	app, err := store.Open()
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer closeDB(app)

	s := NewHostsStore(app)
	id, err := s.SaveHost(HostInput{
		Name: "h", Addr: "1.1.1.1", Port: 22, User: "u",
		AuthType: "password", Secret: "x", AutoApprove: "on",
	})
	if err != nil {
		t.Fatalf("SaveHost: %v", err)
	}
	// 默认：未显式指定时落库为 'inherit'（列默认值）
	id2, _ := s.SaveHost(HostInput{
		Name: "h2", Addr: "1.1.1.2", Port: 22, User: "u",
		AuthType: "password", Secret: "x",
	})
	if v, _ := s.GetAutoApprove(id); v != "on" {
		t.Errorf("GetAutoApprove = %q，want on", v)
	}
	if v, _ := s.GetAutoApprove(id2); v != "inherit" {
		t.Errorf("GetAutoApprove 默认 = %q，want inherit", v)
	}
	if err := s.UpdateHost(id, HostInput{
		Name: "h", Addr: "1.1.1.1", Port: 22, User: "u",
		AuthType: "password", AutoApprove: "off",
	}); err != nil {
		t.Fatalf("UpdateHost: %v", err)
	}
	if v, _ := s.GetAutoApprove(id); v != "off" {
		t.Errorf("UpdateHost 后 GetAutoApprove = %q，want off", v)
	}
}
