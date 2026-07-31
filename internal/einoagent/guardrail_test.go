package einoagent

import "testing"

func TestAssessRisk(t *testing.T) {
	cases := []struct {
		cmd   string
		level string // "" = 无风险, "high"
	}{
		{"ls -la", ""},
		{"top -bn1", ""},
		{"rm -rf /", "high"},
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
