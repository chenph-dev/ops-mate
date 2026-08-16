package connector

import "testing"

func TestIsQuery(t *testing.T) {
	cases := []struct {
		sql  string
		want bool
	}{
		{"SELECT * FROM users", true},
		{"  show tables", true},
		{"DESC users", true},
		{"explain SELECT 1", true},
		{"WITH x AS (SELECT 1) UPDATE t SET a=1", true},
		{"PRAGMA table_info(t)", true},
		{"INSERT INTO t VALUES (1)", false},
		{"update users set name='x'", false},
		{"DELETE FROM users", false},
		{"DROP TABLE users", false},
		{"", false},
	}
	for _, c := range cases {
		if got := IsQuery(c.sql); got != c.want {
			t.Errorf("IsQuery(%q) = %v, want %v", c.sql, got, c.want)
		}
	}
}
