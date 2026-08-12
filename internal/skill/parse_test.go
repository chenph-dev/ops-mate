package skill

import (
	"archive/zip"
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

// buildZip 用 map[path]content 构造内存 ZIP。
func buildZip(files map[string]string) []byte {
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	for name, content := range files {
		f, err := w.Create(name)
		if err != nil {
			panic(err)
		}
		if _, err := f.Write([]byte(content)); err != nil {
			panic(err)
		}
	}
	if err := w.Close(); err != nil {
		panic(err)
	}
	return buf.Bytes()
}

func TestParseFrontmatter_Valid(t *testing.T) {
	md := "---\nname: nginx-check\ndescription: 检查 Nginx 状态\n---\n1. 查看状态\n2. 看日志\n"
	m, err := parseFrontmatter([]byte(md))
	if err != nil {
		t.Fatalf("parseFrontmatter: %v", err)
	}
	if m.Name != "nginx-check" || m.Description != "检查 Nginx 状态" {
		t.Fatalf("Meta: %+v", m)
	}
	if m.Body != "1. 查看状态\n2. 看日志" {
		t.Fatalf("Body: %q", m.Body)
	}
}

func TestParseFrontmatter_UTF8BOM(t *testing.T) {
	// Windows Set-Content -Encoding UTF8 会写 BOM，应兼容。
	md := "\xef\xbb\xbf---\nname: a\ndescription: x\n---\nbody"
	m, err := parseFrontmatter([]byte(md))
	if err != nil {
		t.Fatalf("parseFrontmatter: %v", err)
	}
	if m.Name != "a" || m.Description != "x" || m.Body != "body" {
		t.Fatalf("Meta: %+v", m)
	}
}

func TestParseFrontmatter_Missing(t *testing.T) {
	if _, err := parseFrontmatter([]byte("no frontmatter")); err == nil {
		t.Fatal("无 frontmatter 应报错")
	}
}

func TestParseFrontmatter_MissingName(t *testing.T) {
	if _, err := parseFrontmatter([]byte("---\ndescription: x\n---\nbody")); err == nil {
		t.Fatal("缺 name 应报错")
	}
}

func TestParseFrontmatter_InvalidSlug(t *testing.T) {
	for _, bad := range []string{"NginxCheck", "nginx check", "nginx_check"} {
		md := "---\nname: " + bad + "\n---\nbody"
		if _, err := parseFrontmatter([]byte(md)); err == nil {
			t.Fatalf("name %q 应为非法 slug 报错", bad)
		}
	}
}

func TestExtractZip_Valid(t *testing.T) {
	dst := t.TempDir()
	root, err := extractZip(buildZip(map[string]string{
		"SKILL.md":    "---\nname: a\n---\nbody",
		"scripts/x":   "#!/bin/bash\necho hi",
		"scripts/sub": "x",
	}), dst)
	if err != nil {
		t.Fatalf("extractZip: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "SKILL.md")); err != nil {
		t.Fatalf("SKILL.md 应解压: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "scripts", "x")); err != nil {
		t.Fatalf("scripts/x 应解压: %v", err)
	}
}

func TestExtractZip_ZipSlip(t *testing.T) {
	dst := t.TempDir()
	_, err := extractZip(buildZip(map[string]string{
		"../evil.txt": "x",
	}), dst)
	if err == nil {
		t.Fatal("zip-slip 路径应被拒绝")
	}
}

func TestExtractZip_WindowsBackslashSeparator(t *testing.T) {
	// PowerShell Compress-Archive 用反斜杠做路径分隔符，应归一为 / 正常解压。
	dst := t.TempDir()
	root, err := extractZip(buildZip(map[string]string{
		`disk-check\SKILL.md`:      "---\nname: disk-check\n---\nbody",
		`disk-check\scripts\run.sh`: "#!/bin/bash\n",
	}), dst)
	if err != nil {
		t.Fatalf("extractZip 应接受反斜杠路径: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "SKILL.md")); err != nil {
		t.Fatalf("反斜杠路径应归一解压: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "scripts", "run.sh")); err != nil {
		t.Fatalf("scripts/run.sh 应解压: %v", err)
	}
}

func TestExtractZip_TopLevelDirNormalized(t *testing.T) {
	dst := t.TempDir()
	root, err := extractZip(buildZip(map[string]string{
		"nginx-check/SKILL.md": "---\nname: a\n---\nbody",
	}), dst)
	if err != nil {
		t.Fatalf("extractZip: %v", err)
	}
	if filepath.Base(root) != "nginx-check" {
		t.Fatalf("顶层目录应归一，root=%q", root)
	}
}
