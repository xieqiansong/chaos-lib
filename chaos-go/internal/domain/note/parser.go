package note

import (
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

// 预编译正则（反引号一律写作 \x60 双引号转义形式，避免原始字符串里的反引号解析问题）。
var (
	reFence      = regexp.MustCompile(`(?s)^---[ \t]*\r?\n(.*?)\r?\n---[ \t]*(?:\r?\n|$)(.*)`)
	reH1         = regexp.MustCompile(`(?m)^#\s+(.+?)\s*$`)
	reFenceCode  = regexp.MustCompile("(?s)\\x60\\x60\\x60.*?\\x60\\x60\\x60")
	reInlineCode = regexp.MustCompile("\\x60+([^\\x60]*)\\x60+")
	reImage      = regexp.MustCompile(`!\[([^\]]*)\]\([^)]*\)`)
	reLink       = regexp.MustCompile(`\[([^\]]*)\]\([^)]*\)`)
	reHeading    = regexp.MustCompile(`(?m)^#{1,6}\s+`)
	reEmphasis   = regexp.MustCompile(`\*\*([^*]+)\*\*|__([^_]+)__|\*([^*]+)\*|_([^_]+)_`)
	reHTML       = regexp.MustCompile(`<[^>]+>`)
)

// frontMeta 从 front matter 解析出的元数据。
type frontMeta struct {
	Title string   `yaml:"title"`
	Tags  []string `yaml:"tags"`
}

// splitFrontMatter 切出文件顶部的 YAML front matter。
// 返回 (frontMatter, body, ok)；无 front matter 时 ok=false，body 为全文。
func splitFrontMatter(s string) (string, string, bool) {
	if !strings.HasPrefix(s, "---") {
		return "", s, false
	}
	m := reFence.FindStringSubmatch(s)
	if m == nil {
		return "", s, false
	}
	return m[1], m[2], true
}

// parseFrontMeta 解析 front matter 的标题与标签。
func parseFrontMeta(fm string) frontMeta {
	var m frontMeta
	_ = yaml.Unmarshal([]byte(fm), &m)
	return m
}

// stripMarkdown 把 Markdown 正文转为去标记纯文本（代码块 / 链接 / 图片 / 强调 / HTML 均剥离）。
func stripMarkdown(s string) string {
	s = reFenceCode.ReplaceAllString(s, " ")
	s = reInlineCode.ReplaceAllString(s, "$1")
	s = reImage.ReplaceAllString(s, "$1")
	s = reLink.ReplaceAllString(s, "$1")
	s = reHeading.ReplaceAllString(s, " $1 ")
	s = reEmphasis.ReplaceAllString(s, "$1$2$3$4")
	s = reHTML.ReplaceAllString(s, " ")
	s = strings.ReplaceAll(s, "\n", " ")
	return strings.Join(strings.Fields(s), " ")
}

// extractTitle 由 (frontMatter, body, fallback) 推出标题：
// front matter.title → 首个一级标题 → fallback（去扩展名文件名）。scanner 变体直接调用。
func extractTitle(fm, body, fallback string) string {
	meta := parseFrontMeta(fm)
	if t := strings.TrimSpace(meta.Title); t != "" {
		return t
	}
	// 去除代码块 / 行内代码后再找标题，避免代码里的 # 被误判为标题
	stripped := reFenceCode.ReplaceAllString(body, " ")
	stripped = reInlineCode.ReplaceAllString(stripped, "$1")
	if mm := reH1.FindStringSubmatch(stripped); mm != nil {
		return strings.TrimSpace(mm[1])
	}
	return fallback
}

// ExtractTitle 由完整 content 推出标题（测试与潜在调用方使用）。
func ExtractTitle(content []byte, fallback string) string {
	text := string(content)
	fm, body, ok := splitFrontMatter(text)
	if !ok {
		body = text
	}
	return extractTitle(fm, body, fallback)
}

// PlainText 把 Markdown 正文转为去标记纯文本，供检索与摘要使用；长度截断 64KB。
func PlainText(content []byte) string {
	text := string(content)
	_, body, ok := splitFrontMatter(text)
	if !ok {
		body = text
	}
	s := stripMarkdown(body)
	if len(s) > 65536 {
		s = s[:65536]
	}
	return s
}

// Summarize 取纯文本前 n 字作为摘要。
func Summarize(content []byte, n int) string {
	s := PlainText(content)
	if len(s) > n {
		return s[:n]
	}
	return s
}
