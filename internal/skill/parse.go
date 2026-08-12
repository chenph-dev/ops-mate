package skill

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

const (
	// MaxZipBytes 解压后总字节上限（防 zip bomb）。
	MaxZipBytes = 5 * 1024 * 1024
	// SkillMD SKILL.md 文件名。
	SkillMD = "SKILL.md"
)

// slugRe 合法技能 name：小写字母数字连字符。
var slugRe = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)

// Meta 从 SKILL.md 解析出的元信息。
type Meta struct {
	Name        string
	Description string
	Body        string // frontmatter 之后的正文
}

// parseFrontmatter 解析 "---\nkey: value\n---\nbody"。
func parseFrontmatter(content []byte) (*Meta, error) {
	// 去除 UTF-8 BOM（Windows 编辑器/Set-Content 常写入）
	s := string(bytes.TrimPrefix(content, []byte{0xEF, 0xBB, 0xBF}))
	if !strings.HasPrefix(s, "---") {
		return nil, fmt.Errorf("SKILL.md 缺少 frontmatter（需以 --- 开头）")
	}
	rest := strings.TrimPrefix(s, "---")
	rest = strings.TrimPrefix(rest, "\n")
	closeIdx := strings.Index(rest, "\n---")
	if closeIdx < 0 {
		return nil, fmt.Errorf("frontmatter 缺少结束标记 ---")
	}
	yml := rest[:closeIdx]
	body := rest[closeIdx:]
	body = strings.TrimPrefix(body, "\n---")
	body = strings.TrimPrefix(body, "\n")

	var fm struct {
		Name        string `yaml:"name"`
		Description string `yaml:"description"`
	}
	if err := yaml.Unmarshal([]byte(yml), &fm); err != nil {
		return nil, fmt.Errorf("解析 frontmatter 失败: %w", err)
	}
	if fm.Name == "" {
		return nil, fmt.Errorf("frontmatter 缺少 name 字段")
	}
	if !slugRe.MatchString(fm.Name) {
		return nil, fmt.Errorf("name 需为小写字母数字连字符 slug（当前: %q）", fm.Name)
	}
	return &Meta{Name: fm.Name, Description: fm.Description, Body: strings.TrimSpace(body)}, nil
}

// zipEntry 一条待解压条目，rel 为归一化后的相对路径（统一 / 分隔符）。
type zipEntry struct {
	file *zip.File
	rel  string
}

// extractZip 校验并解压技能 ZIP 到 dst。路径分隔符先归一（兼容 Windows 工具
// 如 PowerShell Compress-Archive 产出的反斜杠条目）。顶层目录归一：若全部条目
// 共用一个顶级目录，则解压到该目录下；否则原样解压到 dst。返回实际技能目录。
func extractZip(zipBytes []byte, dst string) (string, error) {
	zr, err := zip.NewReader(bytes.NewReader(zipBytes), int64(len(zipBytes)))
	if err != nil {
		return "", fmt.Errorf("读取 ZIP 失败: %w", err)
	}

	entries := make([]zipEntry, 0, len(zr.File))
	for _, f := range zr.File {
		rel := strings.ReplaceAll(f.Name, "\\", "/")
		// zip-slip 防护：拒绝 .. 段、绝对路径、盘符路径
		if strings.Contains(rel, "..") || strings.HasPrefix(rel, "/") || filepath.IsAbs(rel) {
			return "", fmt.Errorf("ZIP 含非法路径: %q", f.Name)
		}
		entries = append(entries, zipEntry{file: f, rel: rel})
	}

	// 顶层目录归一：根级已有 SKILL.md 时不归一；否则若全部条目共用一个
	// 顶级目录（如 nginx-check/），解压时去掉该前缀。
	wrapper := ""
	hasRootSkillMD := false
	for _, e := range entries {
		if strings.SplitN(e.rel, "/", 2)[0] == SkillMD {
			hasRootSkillMD = true
			break
		}
	}
	if !hasRootSkillMD {
		for _, e := range entries {
			first := strings.SplitN(e.rel, "/", 2)[0]
			if first == "" {
				continue
			}
			if wrapper == "" {
				wrapper = first
			} else if wrapper != first {
				wrapper = ""
				break
			}
		}
	}
	root := dst
	if wrapper != "" {
		root = filepath.Join(dst, wrapper)
	}

	cleanDst := filepath.Clean(dst)
	var total int64
	for _, e := range entries {
		rel := e.rel
		if wrapper != "" {
			rel = strings.TrimPrefix(rel, wrapper+"/")
		}
		f := e.file
		target := filepath.Join(root, rel)
		cleanTarget := filepath.Clean(target)
		if cleanTarget != cleanDst && !strings.HasPrefix(cleanTarget, cleanDst+string(filepath.Separator)) {
			return "", fmt.Errorf("ZIP 条目逃逸目标目录: %q", f.Name)
		}
		if f.FileInfo().IsDir() {
			if err := os.MkdirAll(target, 0o755); err != nil {
				return "", err
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return "", err
		}
		rc, err := f.Open()
		if err != nil {
			return "", err
		}
		dstFile, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o755)
		if err != nil {
			rc.Close()
			return "", err
		}
		n, err := io.Copy(dstFile, rc)
		rc.Close()
		dstFile.Close()
		if err != nil {
			return "", err
		}
		total += n
		if total > MaxZipBytes {
			return "", fmt.Errorf("ZIP 解压总大小超过 %d 字节", MaxZipBytes)
		}
	}
	return root, nil
}
