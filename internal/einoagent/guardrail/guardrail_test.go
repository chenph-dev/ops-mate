package guardrail

import "testing"

func TestAssessRisk(t *testing.T) {
	cases := []struct {
		cmd   string
		level string // "" = 无风险, "high"
	}{
		{"ls -la", ""},
		{"top -bn1", ""},
		{"rm -rf /", "high"},
		{"rm -rf /*", "high"},
		{"mkfs.ext4 /dev/sda1", "high"},
		{"dd if=/dev/zero of=/dev/sdb", "high"},
		{"shutdown -h now", "high"},
		{"reboot", "high"},
		{":() { :|:& };:", "high"},
		{"echo hi > /dev/sda", "high"},
	}
	for _, c := range cases {
		got := AssessRisk(c.cmd)
		if c.level == "" {
			if got != "" {
				t.Errorf("命令 %q 期望无风险，得到 %q", c.cmd, got)
			}
		} else if got != c.level {
			t.Errorf("命令 %q 期望 %q，得到 %q", c.cmd, c.level, got)
		}
	}
}

func TestClassify(t *testing.T) {
	cases := []struct {
		cmd        string
		whitelist  []string
		wantRisk   string
		wantAction Action
	}{
		{"rm -rf /", nil, "high", ActionApprove},
		{"ls -la", nil, "read", ActionAuto},
		{"df -h", []string{"df"}, "read", ActionAuto},
		{"touch /tmp/x", nil, "write", ActionApprove},
		{"ls | grep x", nil, "write", ActionApprove},
		{"cat a > /tmp/out", nil, "write", ActionApprove},
		{"echo $(ls)", nil, "write", ActionApprove},
		// 显式白名单覆盖：cat 不在 list 中 → write
		{"cat /etc/hosts", []string{"ls", "df"}, "write", ActionApprove},
		// 空白名单回退内置默认 → ls 仍 read
		{"ls -la", []string{}, "read", ActionAuto},
	}
	for _, c := range cases {
		risk, action := Classify(c.cmd, c.whitelist)
		if risk != c.wantRisk || action != c.wantAction {
			t.Errorf("Classify(%q, %v) = (%q, %q)，want (%q, %q)",
				c.cmd, c.whitelist, risk, action, c.wantRisk, c.wantAction)
		}
	}
}

func TestIsReadOnlyCommand(t *testing.T) {
	wl := []string{"ls", "df", "free"}
	cases := []struct {
		cmd  string
		want bool
	}{
		{"ls -la", true},
		{"df -h", true},
		{"free", true},
		{"top -bn1", false},       // top 不在白名单
		{"ls | grep x", false},    // 管道
		{"ls > /tmp/out", false},  // 重定向
		{"ls >> /tmp/out", false}, // 追加重定向
		{"echo $(ls)", false},     // 命令替换
		{"echo `ls`", false},      // 反引号
		{"cat a; ls", false},      // 分号
		{"", false},
	}
	for _, c := range cases {
		if got := IsReadOnlyCommand(c.cmd, wl); got != c.want {
			t.Errorf("IsReadOnlyCommand(%q) = %v，want %v", c.cmd, got, c.want)
		}
	}
}

func TestParseReadOnlyList(t *testing.T) {
	got := ParseReadOnlyList("ls, df ,\nfree,ls,  ,top")
	want := []string{"ls", "df", "free", "top"}
	if len(got) != len(want) {
		t.Fatalf("ParseReadOnlyList 长度 = %d，want %d (%v)", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("ParseReadOnlyList[%d] = %q，want %q", i, got[i], want[i])
		}
	}
}

func TestAssessRisk_UnchangedForClean(t *testing.T) {
	if got := AssessRisk("ls -la"); got != "" {
		t.Errorf("AssessRisk 应保持二态（干净命令返回空串），得到 %q", got)
	}
	if got := AssessRisk("rm -rf /"); got != "high" {
		t.Errorf("AssessRisk 应返回 high，得到 %q", got)
	}
}

func TestAssessRiskForProtocol_Windows(t *testing.T) {
	cases := []struct {
		cmd   string
		proto string
		level string // "" no risk, "high"
	}{
		{"format c:", "winrm", "high"},
		{"del /s /q C:\\temp\\*", "winrm", "high"},
		{"rd /s /q C:\\data", "winrm", "high"},
		{"rmdir /s foo", "winrm", "high"},
		{"shutdown /s /t 0", "winrm", "high"},
		{"shutdown /r", "winrm", "high"},
		{"Remove-Item -Path X -Recurse -Force", "winrm", "high"},
		{"diskpart", "winrm", ""},
		{"select disk 0\nclean", "winrm", "high"},
		{"Get-Process", "winrm", ""},
		{"format c:", "ssh", ""},
		{"del /s /q", "ssh", ""},
		{"Get-Process", "ssh", ""},
		{"format c:", "", ""},
		{"FORMAT C:", "winrm", "high"},
		{"DEL /S /Q", "winrm", "high"},
		{"Remove-Item -Force -Recurse C:\\x", "winrm", "high"},
		{"SHUTDOWN /S", "winrm", "high"},
	}
	for _, c := range cases {
		got := AssessRiskForProtocol(c.cmd, c.proto)
		if c.level == "" {
			if got != "" {
				t.Errorf("command %q (proto=%q) expected no risk, got %q", c.cmd, c.proto, got)
			}
		} else if got != c.level {
			t.Errorf("command %q (proto=%q) expected %q, got %q", c.cmd, c.proto, c.level, got)
		}
	}
}

func TestClassifyForProtocol_WindowsWhitelist(t *testing.T) {
	if risk, action := ClassifyForProtocol("Get-Process", nil, "winrm"); risk != "read" || action != ActionAuto {
		t.Errorf("winrm Get-Process should be read-only auto-approved, got (%q,%q)", risk, action)
	}
	if risk, _ := ClassifyForProtocol("Get-ChildItem", nil, "winrm"); risk != "read" {
		t.Errorf("winrm Get-ChildItem should be read-only, got %q", risk)
	}
	if risk, action := ClassifyForProtocol("format c:", nil, "winrm"); risk != "high" || action != ActionApprove {
		t.Errorf("winrm format should be high-risk approve, got (%q,%q)", risk, action)
	}
	if risk, action := ClassifyForProtocol("Get-Process", nil, "ssh"); risk != "write" || action != ActionApprove {
		t.Errorf("ssh Get-Process should be write (not in Linux whitelist), got (%q,%q)", risk, action)
	}
}

func TestDefaultReadOnlyCommandsWindows_NotEmpty(t *testing.T) {
	if len(DefaultReadOnlyCommandsWindows) == 0 {
		t.Error("Windows read-only whitelist should not be empty")
	}
}

func TestClassifyForProtocol_JdbcSQL(t *testing.T) {
	cases := []struct {
		sql    string
		risk   string
		action Action
	}{
		{"SELECT * FROM users", "read", ActionAuto},
		{"SHOW TABLES", "read", ActionAuto},
		{"DESCRIBE users", "read", ActionAuto},
		{"EXPLAIN SELECT 1", "read", ActionAuto},
		{"USE app", "read", ActionAuto},
		{"INSERT INTO t VALUES (1)", "write", ActionApprove},
		{"UPDATE users SET name='x'", "write", ActionApprove},
		{"DELETE FROM users", "write", ActionApprove},
		{"CREATE TABLE t (id INT)", "write", ActionApprove},
		{"DROP TABLE users", "high", ActionApprove},
		{"TRUNCATE TABLE t", "high", ActionApprove},
		{"ALTER TABLE t ADD COLUMN c INT", "high", ActionApprove},
		{"SELECT 1; DROP TABLE users", "write", ActionApprove},              // 多语句保守审批
		{"WITH x AS (SELECT 1) UPDATE t SET a=1", "write", ActionApprove},  // CTE 写保守审批
		{"SELECT * FROM t INTO OUTFILE '/tmp/x'", "write", ActionApprove},  // 只读关键字但 INTO OUTFILE 写文件
		{"SELECT LOAD_FILE('/etc/passwd')", "write", ActionApprove},        // SELECT fn() 读服务器文件
		{"SELECT * FROM users FOR UPDATE", "write", ActionApprove},         // 只读关键字但 FOR UPDATE 加锁
		{"SELECT * FROM users for update", "write", ActionApprove},         // 小写 FOR UPDATE 同样降级
		{"PRAGMA journal_mode=WAL", "write", ActionApprove},                // PRAGMA 写模式
		{"PRAGMA writable_schema=ON", "write", ActionApprove},              // PRAGMA 危险开关
		{"PRAGMA table_info(users)", "read", ActionAuto},                   // 普通只读 PRAGMA 不受影响
	}
	for _, c := range cases {
		risk, action := ClassifyForProtocol(c.sql, nil, "jdbc")
		if risk != c.risk || action != c.action {
			t.Errorf("ClassifyForProtocol(%q, jdbc) = (%q,%q), want (%q,%q)", c.sql, risk, action, c.risk, c.action)
		}
	}
}

func TestAssessRiskForProtocol_Jdbc(t *testing.T) {
	if AssessRiskForProtocol("SELECT * FROM users", "jdbc") != "" {
		t.Error("jdbc SELECT 不应判高风险")
	}
	if AssessRiskForProtocol("DROP TABLE users", "jdbc") != "high" {
		t.Error("jdbc DROP 应判高风险")
	}
	if AssessRiskForProtocol("UPDATE users SET name='x'", "jdbc") != "" {
		t.Error("jdbc UPDATE 不应判高风险（写操作由审批而非 high 标红）")
	}
	// jdbc 不受 Linux/Windows shell 危险模式影响
	if AssessRiskForProtocol("rm -rf /", "jdbc") != "" {
		t.Error("jdbc 不应套用 shell 危险模式")
	}
}

func TestClassifyForProtocol_SQLProtocol(t *testing.T) {
	cases := []struct {
		sql    string
		risk   string
		action Action
	}{
		{"SELECT * FROM users", "read", ActionAuto},
		{"DROP TABLE users", "high", ActionApprove},
		{"UPDATE users SET name='x'", "write", ActionApprove},
	}
	for _, c := range cases {
		risk, action := ClassifyForProtocol(c.sql, nil, "sql")
		if risk != c.risk || action != c.action {
			t.Errorf("ClassifyForProtocol(%q, sql) = (%q,%q), want (%q,%q)",
				c.sql, risk, action, c.risk, c.action)
		}
	}
	if AssessRiskForProtocol("DROP TABLE users", "sql") != "high" {
		t.Error("sql DROP 应判高风险")
	}
	if AssessRiskForProtocol("rm -rf /", "sql") != "" {
		t.Error("sql 不应套用 shell 危险模式")
	}
}

// SQL 护栏不得被注释/空白变体绕过：INTO/**/OUTFILE、FOR\tUPDATE 等
// 必须降级为人工审批，不能自动放行。
func TestClassifyForProtocol_SQLCommentBypass(t *testing.T) {
	cases := []struct {
		sql    string
		risk   string
		action Action
	}{
		{"SELECT 1 INTO/**/OUTFILE '/tmp/x'", "write", ActionApprove},
		{"SELECT 1 INTO\tOUTFILE '/tmp/x'", "write", ActionApprove},
		{"SELECT 1 INTO\nOUTFILE '/tmp/x'", "write", ActionApprove},
		{"SELECT LOAD/**/FILE('/etc/passwd')", "write", ActionApprove},
		{"SELECT * FROM users FOR/**/UPDATE", "write", ActionApprove},
		{"SELECT * FROM users FOR\tUPDATE", "write", ActionApprove},
		{"SEL/**/ECT 1 INTO OUTFILE '/tmp/x'", "write", ActionApprove}, // 关键字内注释
		{"SELECT 1 -- 普通行注释\n", "read", ActionAuto},               // 正常行注释保持只读
		{"SELECT 1 /* 普通块注释 */", "read", ActionAuto},              // 正常块注释保持只读
		{"SELECT 1; -- 尾随注释", "write", ActionApprove},              // 剥离注释后真分号仍判审批
		{"SELECT 1 /*;*/", "read", ActionAuto},                         // 注释内分号不误报
	}
	for _, c := range cases {
		risk, action := ClassifyForProtocol(c.sql, nil, "sql")
		if risk != c.risk || action != c.action {
			t.Errorf("ClassifyForProtocol(%q, sql) = (%q,%q), want (%q,%q)",
				c.sql, risk, action, c.risk, c.action)
		}
	}
}

// 只读命令自动放行不得触及凭据/密钥文件：命中白名单但参数含敏感路径 → 降级审批。
func TestIsReadOnlyCommand_SensitiveArgs(t *testing.T) {
	cases := []struct {
		cmd  string
		want bool
	}{
		{"cat /etc/shadow", false},          // 口令文件
		{"cat /etc/sh\\adow", false},        // 反斜杠转义绕过
		{"cat /etc/sh''adow", false},        // 空引号拼接绕过
		{"cat ~/.ssh/id_rsa", false},        // SSH 私钥
		{"cat /root/.ssh/authorized_keys", false},
		{"ls -la /root/.ssh", false},        // 敏感目录
		{"cat /root/.kube/config", false},   // kubeconfig
		{"cat /root/.aws/credentials", false},
		{"grep secret /home/u/.env", false}, // .env 凭据文件
		{"cat /etc/hosts", true},            // 非敏感路径
		{"cat /var/log/app.log", true},      // 日志正常读
		{"ls -la /var/log", true},
	}
	for _, c := range cases {
		if got := IsReadOnlyCommand(c.cmd, DefaultReadOnlyCommands); got != c.want {
			t.Errorf("IsReadOnlyCommand(%q) = %v, want %v", c.cmd, got, c.want)
		}
	}
}

// env 直接从默认白名单移除：输出全部环境变量（可能含密钥），不应自动放行。
func TestDefaultReadOnlyCommands_NoEnv(t *testing.T) {
	for _, w := range DefaultReadOnlyCommands {
		if w == "env" {
			t.Error("默认只读白名单不应包含 env（直接泄露全部环境变量）")
		}
	}
}
