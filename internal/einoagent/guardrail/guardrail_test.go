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
