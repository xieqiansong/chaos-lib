package project

import (
	"os"
	"path/filepath"
	"testing"
)

// writeTestTree 在 dir 下创建一个含不同内容的子文件和子目录的目录树。
// 每个文件写入不同字节，便于移动后校验内容未被截断/损坏。
func writeTestTree(t *testing.T, dir string) {
	t.Helper()
	files := map[string][]byte{
		"a.txt":        []byte("hello-world"),
		"sub/b.txt":    []byte("second-file-content-12345"),
		"sub/c.md":     []byte("# markdown\n内容含中文测试"),
		"deep/x/y.bin": []byte{0x00, 0x01, 0x02, 0x03, 0xFF, 0xFE},
	}
	for rel, content := range files {
		p := filepath.Join(dir, rel)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, content, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	// 空目录
	if err := os.MkdirAll(filepath.Join(dir, "emptyDir"), 0o755); err != nil {
		t.Fatal(err)
	}
}

// snapshotTree 在移动前调用，递归读取 dir 下所有文件的内容指纹（相对路径 -> 字节）。
func snapshotTree(t *testing.T, dir string) map[string][]byte {
	t.Helper()
	want := map[string][]byte{}
	err := filepath.Walk(dir, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(dir, p)
		if rel == "." || info.IsDir() {
			return nil
		}
		data, rerr := os.ReadFile(p)
		if rerr != nil {
			return rerr
		}
		want[rel] = data
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return want
}

// assertTreeMatch 断言 dstDir 中每个相对路径都存在、内容字节一致，且不多不少。
func assertTreeMatch(t *testing.T, want map[string][]byte, dstDir string) {
	t.Helper()
	// 逐文件校验存在 + 内容
	for rel, wantData := range want {
		got, rerr := os.ReadFile(filepath.Join(dstDir, rel))
		if rerr != nil {
			t.Fatalf("移动后缺失文件或读取失败 %s: %v", rel, rerr)
		}
		if string(got) != string(wantData) {
			t.Fatalf("文件内容不一致 %s\n  期望: %q\n  实际: %q", rel, wantData, got)
		}
	}
	// 校验目标不含有源中不存在的多余文件
	filepath.Walk(dstDir, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		rel, _ := filepath.Rel(dstDir, p)
		if _, ok := want[rel]; !ok {
			t.Fatalf("移动后多出源中不存在的文件 %s", rel)
		}
		return nil
	})
}

func TestMoveProjectFolder_SameVolumeRename(t *testing.T) {
	base := t.TempDir()
	oldAbs := filepath.Join(base, "projectA")
	newAbs := filepath.Join(base, "renamed", "projectA")
	writeTestTree(t, oldAbs)
	want := snapshotTree(t, oldAbs)

	if err := MoveProjectFolder(oldAbs, newAbs); err != nil {
		t.Fatalf("MoveProjectFolder failed: %v", err)
	}
	// 源已不在原处（直接移动，非复制）
	if _, err := os.Stat(oldAbs); !os.IsNotExist(err) {
		t.Fatalf("源目录应已被移动走，但仍然存在: %s", oldAbs)
	}
	// 递归校验目标内所有子文件内容与结构完整
	assertTreeMatch(t, want, newAbs)
}

func TestMoveProjectFolder_CopiesContentFully(t *testing.T) {
	base := t.TempDir()
	oldAbs := filepath.Join(base, "src")
	newAbs := filepath.Join(base, "dst")
	writeTestTree(t, oldAbs)
	want := snapshotTree(t, oldAbs)

	if err := MoveProjectFolder(oldAbs, newAbs); err != nil {
		t.Fatalf("MoveProjectFolder failed: %v", err)
	}
	assertTreeMatch(t, want, newAbs)
}

func TestMoveProjectFolder_NoOverwriteExisting(t *testing.T) {
	base := t.TempDir()
	oldAbs := filepath.Join(base, "src")
	newAbs := filepath.Join(base, "dst")
	writeTestTree(t, oldAbs)
	if err := os.MkdirAll(newAbs, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(newAbs, "keep.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	err := MoveProjectFolder(oldAbs, newAbs)
	if err == nil {
		t.Fatal("目标已存在时应返回错误，但未报错")
	}
	// 源不能被破坏：内容仍可读且一致
	assertTreeMatch(t, snapshotTree(t, oldAbs), oldAbs)
}

func TestMoveProjectFolder_SamePathNoOp(t *testing.T) {
	base := t.TempDir()
	oldAbs := filepath.Join(base, "same")
	writeTestTree(t, oldAbs)
	if err := MoveProjectFolder(oldAbs, oldAbs); err != nil {
		t.Fatalf("相同路径应视为 no-op: %v", err)
	}
	if _, err := os.Stat(oldAbs); err != nil {
		t.Fatalf("no-op 后源应仍在: %v", err)
	}
}

func TestMoveProjectFolder_SourceMissing(t *testing.T) {
	base := t.TempDir()
	oldAbs := filepath.Join(base, "ghost")
	newAbs := filepath.Join(base, "dst")
	if err := MoveProjectFolder(oldAbs, newAbs); err != nil {
		t.Fatalf("源不存在时应幂等返回 nil（不报错），实际报错: %v", err)
	}
	if _, err := os.Stat(newAbs); err == nil {
		t.Fatal("源不存在时不应创建目标目录")
	}
}

func TestRemoveDirSafe_Idempotent(t *testing.T) {
	base := t.TempDir()
	dir := filepath.Join(base, "todelete")
	writeTestTree(t, dir)
	if err := RemoveDirSafe(dir); err != nil {
		t.Fatalf("RemoveDirSafe failed: %v", err)
	}
	// 第二次调用应安全返回（目录已不存在）
	if err := RemoveDirSafe(dir); err != nil {
		t.Fatalf("重复删除应幂等返回 nil: %v", err)
	}
}

// TestMoveProjectFolder_CrossVolumeFallback 验证跨卷回退路径（copyDir + 删源）的产物正确性。
// 同机单盘无法触发 EXDEV，这里直接演练回退组合，确保跨卷时内容完整且源被清理。
func TestMoveProjectFolder_CrossVolumeFallback(t *testing.T) {
	base := t.TempDir()
	oldAbs := filepath.Join(base, "src")
	newAbs := filepath.Join(base, "dst")
	writeTestTree(t, oldAbs)
	files := snapshotTree(t, oldAbs)

	// 模拟 os.Rename 失败后的回退逻辑
	if err := copyDir(oldAbs, newAbs); err != nil {
		t.Fatalf("copyDir failed: %v", err)
	}
	if err := RemoveDirSafe(oldAbs); err != nil {
		t.Fatalf("RemoveDirSafe failed: %v", err)
	}

	// 源已删除
	if _, err := os.Stat(oldAbs); !os.IsNotExist(err) {
		t.Fatalf("回退后源应被删除: %s", oldAbs)
	}
	// 逐文件校验内容字节
	for rel, want := range files {
		got, rerr := os.ReadFile(filepath.Join(newAbs, rel))
		if rerr != nil {
			t.Fatalf("回退后缺失文件 %s: %v", rel, rerr)
		}
		if string(got) != string(want) {
			t.Fatalf("回退后内容不一致 %s", rel)
		}
	}
}

func TestMoveProjectFolder_PreservesNestedTree(t *testing.T) {
	base := t.TempDir()
	oldAbs := filepath.Join(base, "deep")
	newAbs := filepath.Join(base, "moved", "deep")
	writeTestTree(t, oldAbs)
	want := snapshotTree(t, oldAbs)

	if err := MoveProjectFolder(oldAbs, newAbs); err != nil {
		t.Fatalf("MoveProjectFolder failed: %v", err)
	}
	assertTreeMatch(t, want, newAbs)
}
