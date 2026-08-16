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

	list, err := s.ListTree()
	if err != nil {
		t.Fatalf("ListTree: %v", err)
	}
	if len(list) != 1 || list[0].Name != "web-01" {
		t.Fatalf("ListTree 结果: %+v", list)
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
	list, _ = s.ListTree()
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
	list, _ := s.ListTree()
	if len(list) != 1 || list[0].Name != "生产环境" {
		t.Fatal("级联删除后应只剩空的生产环境目录")
	}
	if len(list[0].Children) != 0 {
		t.Fatal("级联删除后资产应消失")
	}
}

func TestHostTree_NestedFolder(t *testing.T) {
	t.Setenv("APPDATA", t.TempDir())
	app, err := store.Open()
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer closeDB(app)

	s := NewHostsStore(app)

	// 三层嵌套：根目录 -> 子目录 -> 孙目录 -> 资产
	rootID, err := s.CreateFolder("根目录", "")
	if err != nil {
		t.Fatalf("CreateFolder(root): %v", err)
	}
	subID, err := s.CreateFolder("子目录", rootID)
	if err != nil {
		t.Fatalf("CreateFolder(sub): %v", err)
	}
	grandID, err := s.CreateFolder("孙目录", subID)
	if err != nil {
		t.Fatalf("CreateFolder(grand): %v", err)
	}
	if _, err := s.SaveHost(HostInput{
		Name: "web-01", ParentID: grandID, Addr: "10.0.0.5",
		Port: 22, User: "ops", AuthType: "password", Secret: "p@ss",
	}); err != nil {
		t.Fatalf("SaveHost: %v", err)
	}

	// 根目录下同时平铺一个资产，验证 host 平铺正确
	if _, err := s.SaveHost(HostInput{
		Name: "root-host", ParentID: rootID, Addr: "10.0.0.6",
		Port: 22, User: "ops", AuthType: "password", Secret: "p@ss",
	}); err != nil {
		t.Fatalf("SaveHost(root-host): %v", err)
	}

	tree, err := s.ListTree()
	if err != nil {
		t.Fatalf("ListTree: %v", err)
	}
	if len(tree) != 1 {
		t.Fatalf("根级应有 1 个目录，得到 %d", len(tree))
	}
	root := tree[0]
	if root.Name != "根目录" {
		t.Errorf("根节点名 = %q, want 根目录", root.Name)
	}
	// 根目录下应有 1 个子目录 + 1 个平铺 host
	if len(root.Children) != 2 {
		t.Fatalf("根目录下应有 2 个子节点（子目录 + 平铺 host），得到 %d: %+v", len(root.Children), root.Children)
	}

	var sub *TreeNode
	for i := range root.Children {
		if root.Children[i].Name == "子目录" {
			sub = &root.Children[i]
		}
	}
	if sub == nil {
		t.Fatal("根目录下未找到子目录")
	}
	if len(sub.Children) != 1 || sub.Children[0].Name != "孙目录" {
		t.Fatalf("子目录下应有 1 个孙目录，得到 %+v", sub.Children)
	}
	// 断言孙节点存在（修复前：值拷贝导致孙节点丢失）
	grand := sub.Children[0]
	if len(grand.Children) != 1 || grand.Children[0].Name != "web-01" {
		t.Fatalf("孙目录下应有 1 个资产（孙节点丢失），得到 %+v", grand.Children)
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

func TestHostParams_Roundtrip(t *testing.T) {
	t.Setenv("APPDATA", t.TempDir())
	app, err := store.Open()
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer closeDB(app)

	s := NewHostsStore(app)

	// sqlite：无 host，只填 filePath
	id, err := s.SaveHost(HostInput{
		Name: "local.db", Protocol: "sqlite",
		Params: map[string]any{"filePath": "C:\\data\\app.db"},
	})
	if err != nil {
		t.Fatalf("SaveHost(sqlite): %v", err)
	}
	meta, err := s.HostMetaByID(id)
	if err != nil {
		t.Fatalf("HostMetaByID: %v", err)
	}
	if meta.Protocol != "sqlite" {
		t.Errorf("protocol = %q, want sqlite", meta.Protocol)
	}
	if meta.Params == nil || meta.Params["filePath"] != "C:\\data\\app.db" {
		t.Errorf("Params 未正确回读: %+v", meta.Params)
	}

	// mysql：params.database 往返
	id2, err := s.SaveHost(HostInput{
		Name: "appdb", Protocol: "mysql", Addr: "10.0.0.6", Port: 3306,
		User: "root", AuthType: "password", Secret: "x",
		Params: map[string]any{"database": "app"},
	})
	if err != nil {
		t.Fatalf("SaveHost(mysql): %v", err)
	}
	meta2, _ := s.HostMetaByID(id2)
	if meta2.Params["database"] != "app" {
		t.Errorf("mysql Params.database = %v, want app", meta2.Params["database"])
	}

	// ListTree 也带出 Params
	tree, _ := s.ListTree()
	for _, n := range tree {
		if n.ID == id2 && n.Params["database"] != "app" {
			t.Errorf("ListTree Params.database = %v, want app", n.Params["database"])
		}
	}

	// UpdateHost 更新 Params
	if err := s.UpdateHost(id2, HostInput{
		Name: "appdb", Protocol: "mysql", Addr: "10.0.0.6", Port: 3306,
		User: "root", AuthType: "password",
		Params: map[string]any{"database": "other"},
	}); err != nil {
		t.Fatalf("UpdateHost: %v", err)
	}
	meta2, _ = s.HostMetaByID(id2)
	if meta2.Params["database"] != "other" {
		t.Errorf("UpdateHost 后 Params.database = %v, want other", meta2.Params["database"])
	}
}

func TestHostKeyFingerprint_Roundtrip(t *testing.T) {
	t.Setenv("APPDATA", t.TempDir())
	app, err := store.Open()
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer closeDB(app)

	s := NewHostsStore(app)
	id, err := s.SaveHost(HostInput{
		Name: "linux01", Addr: "10.0.0.10", Port: 22, User: "root",
		AuthType: "password", Secret: "x", Protocol: "ssh",
		Params: map[string]any{"database": "app"},
	})
	if err != nil {
		t.Fatalf("SaveHost: %v", err)
	}

	// 初始无信任记录
	fp, err := s.HostKeyFingerprint(id)
	if err != nil || fp != "" {
		t.Fatalf("初始指纹应为空，得到 %q, %v", fp, err)
	}

	// 首次连接写入指纹，且不覆盖既有 params
	const wantFP = "SHA256:abc123"
	if err := s.SaveHostKeyFingerprint(id, wantFP); err != nil {
		t.Fatalf("SaveHostKeyFingerprint: %v", err)
	}
	got, err := s.HostKeyFingerprint(id)
	if err != nil || got != wantFP {
		t.Fatalf("指纹回读 = %q, %v; want %q", got, err, wantFP)
	}
	meta, err := s.HostMetaByID(id)
	if err != nil {
		t.Fatalf("HostMetaByID: %v", err)
	}
	if meta.Params["database"] != "app" {
		t.Errorf("写入指纹不应覆盖既有 params: %+v", meta.Params)
	}
}
