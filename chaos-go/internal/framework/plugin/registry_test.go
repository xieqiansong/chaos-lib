package plugin

import "testing"

// reset 清空全局登记，保证用例间互不干扰（测试与被测同包，可直接访问未导出 regs）。
func reset() { regs = nil }

func TestBuildMissingDependency(t *testing.T) {
	reset()
	Register(Registration{Meta: Meta{ID: "a", Dependencies: []string{"ghost"}}})
	if _, err := Build(); err == nil {
		t.Fatal("期望缺失依赖报错，但未报错")
	}
}

func TestBuildCycle(t *testing.T) {
	reset()
	Register(Registration{Meta: Meta{ID: "a", Dependencies: []string{"b"}}})
	Register(Registration{Meta: Meta{ID: "b", Dependencies: []string{"a"}}})
	if _, err := Build(); err == nil {
		t.Fatal("期望循环依赖报错，但未报错")
	}
}

func TestBuildDependencyOrder(t *testing.T) {
	reset()
	// b 无依赖；a 依赖 b；c 依赖 a。期望 b → a → c。
	Register(Registration{Meta: Meta{ID: "a", Dependencies: []string{"b"}}})
	Register(Registration{Meta: Meta{ID: "b"}})
	Register(Registration{Meta: Meta{ID: "c", Dependencies: []string{"a"}}})

	out, err := Build()
	if err != nil {
		t.Fatalf("不应报错: %v", err)
	}
	want := []string{"b", "a", "c"}
	if len(out) != len(want) {
		t.Fatalf("插件数量不符: %v", out)
	}
	for i, id := range want {
		if out[i].Meta.ID != id {
			t.Fatalf("拓扑序错误：期望第 %d 位为 %q，实际 %q", i, id, out[i].Meta.ID)
		}
	}
}

func TestModelsOf(t *testing.T) {
	reset()
	type m1 struct{ ID int }
	type m2 struct{ ID int }
	Register(Registration{Meta: Meta{ID: "x"}, Models: []any{&m1{}}})
	Register(Registration{Meta: Meta{ID: "y"}, Models: []any{&m2{}}})
	regs, _ := Build()
	if got := len(ModelsOf(regs)); got != 2 {
		t.Fatalf("ModelsOf 应汇总 2 个模型，实际 %d", got)
	}
}
