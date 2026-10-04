package note

import (
	"errors"
	"strings"
	"testing"
)

// 路径穿越防护是整套模块的安全底线，必须守住。
func TestResolveSafePath(t *testing.T) {
	// 用真实绝对路径作 root（Windows 下 filepath.Join 产出反斜杠，与生产一致）。
	root := t.TempDir()
	cases := []struct {
		rel     string
		wantErr bool
	}{
		{"", false},
		{"a/b.md", false},
		{"a/../b.md", true},   // 含 .. 一律拒绝（真实 relPath 永不含 ..，安全优先）
		{"../escape.md", true}, // 跳出根目录
		{"a/../../escape.md", true},
		{"/etc/passwd", true}, // 绝对路径输入
		{"a/./b.md", false},   // 规范化
		{"\x00bad", true},     // 非法字符
	}
	for _, c := range cases {
		_, err := resolveSafePath(root, c.rel)
		if c.wantErr {
			if !errors.Is(err, ErrInvalidPath) && !errors.Is(err, ErrPathEscapesVault) {
				t.Errorf("resolveSafePath(%q) 期望错误，得到 %v", c.rel, err)
			}
			continue
		}
		if err != nil {
			t.Errorf("resolveSafePath(%q) 意外错误 %v", c.rel, err)
		}
	}
}

func TestExtractTitle(t *testing.T) {
	withFM := []byte("---\ntitle: 真实标题\n---\n# 这是标题吗\n正文")
	if got := ExtractTitle(withFM, "兜底"); got != "真实标题" {
		t.Errorf("front matter 标题解析失败：%q", got)
	}

	noFM := []byte("# 一级标题\n正文")
	if got := ExtractTitle(noFM, "兜底"); got != "一级标题" {
		t.Errorf("一级标题解析失败：%q", got)
	}

	// 代码块内的 # 不应被当作标题
	inCode := []byte("```\n# 这是代码\n```\n正文")
	if got := ExtractTitle(inCode, "兜底名"); got != "兜底名" {
		t.Errorf("代码块内 # 不应成为标题：%q", got)
	}
}

func TestPlainText(t *testing.T) {
	md := []byte("# 标题\n\n这是 **加粗** 和 [链接](http://x.com) 与 `代码`。\n<!-- 注释 -->")
	got := PlainText(md)
	for _, bad := range []string{"**", "http://x.com", "`代码`", "<!--", " -->"} {
		if strings.Contains(got, bad) {
			t.Errorf("PlainText 未剥离 %q：%q", bad, got)
		}
	}
	if !strings.Contains(got, "标题") || !strings.Contains(got, "加粗") {
		t.Errorf("PlainText 丢失正文：%q", got)
	}
	if len(got) > 65536 {
		t.Errorf("PlainText 未截断：%d", len(got))
	}
}
