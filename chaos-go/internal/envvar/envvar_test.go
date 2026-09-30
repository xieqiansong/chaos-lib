package envvar

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

// ── TOML 往返 ──

func TestEnvTOMLRoundTrip(t *testing.T) {
	snap := &EnvSnapshot{
		Meta:   EnvMeta{SavedAt: "2026-01-02T03:04:05Z", Hostname: "chaos-pc", Username: "xqs"},
		System: EnvSection{"Path": "C:/tools/bin;/usr/bin", "JAVA_HOME": "C:/jdk"},
		User:   EnvSection{"EDITOR": "vim", "空值键": ""},
	}

	text, err := MarshalEnvToTOML(snap)
	if err != nil {
		t.Fatalf("MarshalEnvToTOML 报错: %v", err)
	}
	// saved_at 已显式指定，不应被自动值覆盖
	if !strings.Contains(text, "2026-01-02T03:04:05Z") {
		t.Fatalf("已指定的 saved_at 丢失或被改写：\n%s", text)
	}

	got, err := ParseEnvFromTOML(text)
	if err != nil {
		t.Fatalf("ParseEnvFromTOML 报错: %v", err)
	}
	if got.Meta != snap.Meta {
		t.Fatalf("meta 不一致：%+v", got.Meta)
	}
	if !reflect.DeepEqual(got.System, snap.System) {
		t.Fatalf("system 不一致：%#v", got.System)
	}
	if !reflect.DeepEqual(got.User, snap.User) {
		t.Fatalf("user 不一致：%#v", got.User)
	}
}

func TestMarshalEnvToTOMLFillsDefaults(t *testing.T) {
	text, err := MarshalEnvToTOML(&EnvSnapshot{})
	if err != nil {
		t.Fatalf("MarshalEnvToTOML 报错: %v", err)
	}
	if !strings.Contains(text, "saved_at") {
		t.Fatalf("saved_at 应自动填充：\n%s", text)
	}

	got, err := ParseEnvFromTOML(text)
	if err != nil {
		t.Fatalf("ParseEnvFromTOML 报错: %v", err)
	}
	// 反序列化后两个 section 必须是非 nil 的空表，否则调用方取键会 panic 风险
	if got.System == nil || got.User == nil {
		t.Fatal("空快照解析后 system/user 不应为 nil")
	}
}

func TestParseEnvFromTOMLSpecialValues(t *testing.T) {
	// 非 ASCII 键必须加引号才符合 TOML 规范
	text := "[meta]\nsaved_at = \"2026-01-01T00:00:00Z\"\n\n[system]\nPath = \"a;b\\\"c\\n d\"\n\n[user]\n\"汉\" = \"值\"\n"
	got, err := ParseEnvFromTOML(text)
	if err != nil {
		t.Fatalf("ParseEnvFromTOML 报错: %v", err)
	}
	if got.System["Path"] != "a;b\"c\n d" {
		t.Fatalf("含引号/换行的值往返失败：%q", got.System["Path"])
	}
	if got.User["汉"] != "值" {
		t.Fatalf("非 ASCII 键值往返失败：%#v", got.User)
	}
}

func TestParseEnvFromTOMLInvalid(t *testing.T) {
	if _, err := ParseEnvFromTOML("这不是 TOML =="); err == nil {
		t.Fatal("非法 TOML 应报错")
	} else if !strings.Contains(err.Error(), "TOML解析失败") {
		t.Fatalf("错误信息应包裹上下文，实际 %v", err)
	}
}

func TestEnvSnapshotToJSON(t *testing.T) {
	data, err := EnvSnapshotToJSON(&EnvSnapshot{
		Meta:   EnvMeta{SavedAt: "2026-01-01T00:00:00Z"},
		System: EnvSection{"A": "1"},
		User:   EnvSection{"B": "2"},
	})
	if err != nil {
		t.Fatalf("EnvSnapshotToJSON 报错: %v", err)
	}
	// EnvSnapshot 只声明了 toml tag，JSON 走的是 Go 字段名；
	// 落库后再反序列化回同一结构是自洽的，这里固定住这个契约。
	var got EnvSnapshot
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("输出不是合法 JSON: %v", err)
	}
	if got.Meta.SavedAt != "2026-01-01T00:00:00Z" {
		t.Fatalf("JSON 往返后 saved_at 丢失：%s", data)
	}
	if !reflect.DeepEqual(got.System, EnvSection{"A": "1"}) || !reflect.DeepEqual(got.User, EnvSection{"B": "2"}) {
		t.Fatalf("JSON 往返后分区内容不符：%s", data)
	}
}

// ── 分区打补丁 ──

func TestApplySectionPatchSetAndUnset(t *testing.T) {
	sec := EnvSection{"A": "1", "B": "2"}
	ApplySectionPatch(&sec, &EnvSectionPatch{
		Set:   map[string]string{"B": "22", "C": "3"},
		Unset: []string{"A", "MISSING"},
	})

	want := EnvSection{"B": "22", "C": "3"}
	if !reflect.DeepEqual(sec, want) {
		t.Fatalf("打补丁结果不符：%#v", sec)
	}
}

func TestApplySectionPatchNilSectionAndNilPatch(t *testing.T) {
	var sec EnvSection
	ApplySectionPatch(&sec, nil)
	if sec != nil {
		t.Fatalf("nil patch 不应初始化分区：%#v", sec)
	}

	ApplySectionPatch(&sec, &EnvSectionPatch{Set: map[string]string{"A": "1"}})
	if sec["A"] != "1" {
		t.Fatalf("nil 分区应先初始化再写入：%#v", sec)
	}
}

func TestApplySectionPatchPathOperations(t *testing.T) {
	start := func() EnvSection { return EnvSection{"Path": "C:/a; C:/b ;C:/c"} }

	t.Run("前置与追加", func(t *testing.T) {
		sec := start()
		ApplySectionPatch(&sec, &EnvSectionPatch{Path: EnvPathPatch{
			Prepend: []string{"C:/head"},
			Append:  []string{"C:/tail"},
		}})
		if got, want := sec["Path"], "C:/head;C:/a;C:/b;C:/c;C:/tail"; got != want {
			t.Fatalf("Path = %q, want %q", got, want)
		}
	})

	t.Run("移除命中项并去重trim后的分隔符", func(t *testing.T) {
		sec := start()
		ApplySectionPatch(&sec, &EnvSectionPatch{Path: EnvPathPatch{
			Remove: []string{"C:/b", "不存在"},
		}})
		if got, want := sec["Path"], "C:/a;C:/c"; got != want {
			t.Fatalf("Path = %q, want %q", got, want)
		}
	})

	t.Run("Remove 区分大小写且按项精确匹配", func(t *testing.T) {
		sec := EnvSection{"Path": "C:/a;C:/A"}
		ApplySectionPatch(&sec, &EnvSectionPatch{Path: EnvPathPatch{Remove: []string{"c:/a"}}})
		if got, want := sec["Path"], "C:/a;C:/A"; got != want {
			t.Fatalf("Path = %q, want %q", got, want)
		}
	})

	t.Run("Replace 优先于 Prepend/Append/Remove", func(t *testing.T) {
		sec := start()
		ApplySectionPatch(&sec, &EnvSectionPatch{Path: EnvPathPatch{
			Replace: []string{"C:/x", "C:/x", "C:/y"},
			Prepend: []string{"C:/ignored"},
			Remove:  []string{"C:/a"},
		}})
		if got, want := sec["Path"], "C:/x;C:/y"; got != want {
			t.Fatalf("Path = %q, want %q（Replace 应整体替换并去重）", got, want)
		}
	})

	t.Run("原本没有 Path 时也能写入", func(t *testing.T) {
		sec := EnvSection{}
		ApplySectionPatch(&sec, &EnvSectionPatch{Path: EnvPathPatch{Append: []string{"C:/new"}}})
		if sec["Path"] != "C:/new" {
			t.Fatalf("Path = %q", sec["Path"])
		}
	})
}

// ── 字符串工具 ──

func TestSplitAndTrim(t *testing.T) {
	cases := []struct {
		in   string
		want []string
	}{
		{"", []string{}},
		{"a;b;c", []string{"a", "b", "c"}},
		{" a ; \tb\t ;c", []string{"a", "b", "c"}},
		{"a;;b;", []string{"a", "b"}}, // 空项与尾随分隔符被忽略
		{";", []string{}},
		{"single", []string{"single"}},
	}
	for _, c := range cases {
		if got := splitAndTrim(c.in, ";"); !reflect.DeepEqual(got, c.want) {
			t.Fatalf("splitAndTrim(%q) = %#v, want %#v", c.in, got, c.want)
		}
	}
}

func TestJoinNonEmpty(t *testing.T) {
	cases := []struct {
		in   []string
		want string
	}{
		{[]string{"a", "b"}, "a;b"},
		{[]string{"", "a", "", "b"}, "a;b"},
		{[]string{"", ""}, ""},
		{nil, ""},
	}
	for _, c := range cases {
		if got := joinNonEmpty(c.in, ";"); got != c.want {
			t.Fatalf("joinNonEmpty(%#v) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestDedupedCopy(t *testing.T) {
	if got := dedupedCopy([]string{"a", "b", "a", "", "b"}); !reflect.DeepEqual(got, []string{"a", "b"}) {
		t.Fatalf("dedupedCopy = %#v", got)
	}
	if got := dedupedCopy(nil); !reflect.DeepEqual(got, []string{}) {
		t.Fatalf("dedupedCopy(nil) = %#v, want 空切片", got)
	}
}

func TestTrimSpaces(t *testing.T) {
	if got := trimSpaces(" \t\r\n a b \n"); got != "a b" {
		t.Fatalf("trimSpaces = %q", got)
	}
	if got := trimSpaces("   "); got != "" {
		t.Fatalf("全空白应返回空串，实际 %q", got)
	}
}

func TestCloneMap(t *testing.T) {
	src := map[string]string{"A": "1"}
	got := cloneMap(src)
	if !reflect.DeepEqual(got, src) {
		t.Fatalf("cloneMap = %#v", got)
	}
	got["A"] = "changed"
	if src["A"] != "1" {
		t.Fatal("cloneMap 应为深拷贝（独立底层 map）")
	}
}
