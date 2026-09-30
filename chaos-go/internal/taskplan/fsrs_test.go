package taskplan

import (
	"strings"
	"testing"
	"time"
)

// ── 小工具 ──

func TestFsrsStepConversion(t *testing.T) {
	cases := map[string]int{
		"1m":    1,
		"45m":   45,
		"1h":    60,
		"2h":    120,
		"1d":    1440,
		"10d":   14400,
		"0m":    0, // 负数与 0 一律按 0 处理
		"-1m":   0,
		"5w":    0, // 未知单位
		"x":     0, // 不可解析
		"":      0,
		"m":     0,
		"1mins": 0, // 单位必须是单字符
	}
	for in, want := range cases {
		if got := convertStepToMinutes(in); got != want {
			t.Fatalf("convertStepToMinutes(%q) = %d, want %d", in, got, want)
		}
	}
}

func TestFsrsMathHelpers(t *testing.T) {
	if got := clamp(15.0, 1, 10); got != 10 {
		t.Fatalf("clamp 上界失效：%v", got)
	}
	if got := clamp(-5.0, 1, 10); got != 1 {
		t.Fatalf("clamp 下界失效：%v", got)
	}
	if got := clamp(5.0, 1, 10); got != 5 {
		t.Fatalf("clamp 改变了区间内的值：%v", got)
	}
	if minInt(3, 7) != 3 || maxInt(3, 7) != 7 {
		t.Fatal("minInt / maxInt 行为不符")
	}
}

func TestFsrsDateDiffInDays(t *testing.T) {
	base := time.Date(2026, 3, 10, 23, 59, 0, 0, time.Local)

	cases := []struct {
		name string
		to   time.Time
		want int
	}{
		{"同一天不算差（23:59 与当天 00:01）", time.Date(2026, 3, 10, 0, 1, 0, 0, time.Local), 0},
		{"跨一天", time.Date(2026, 3, 11, 0, 1, 0, 0, time.Local), 1},
		{"跨十天（忽略时刻）", time.Date(2026, 3, 20, 12, 0, 0, 0, time.Local), 10},
		{"早于基准为负", time.Date(2026, 3, 9, 12, 0, 0, 0, time.Local), -1},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := dateDiffInDays(base, c.to); got != c.want {
				t.Fatalf("dateDiffInDays = %d, want %d", got, c.want)
			}
		})
	}
}

func TestFsrsDateScheduler(t *testing.T) {
	now := time.Date(2026, 3, 10, 8, 30, 0, 0, time.Local)
	if got := dateScheduler(now, 3, true); !got.Equal(now.Add(72 * time.Hour)) {
		t.Fatalf("按天调度的间隔不符：%v", got)
	}
	if got := dateScheduler(now, 30, false); !got.Equal(now.Add(30 * time.Minute)) {
		t.Fatalf("按分钟调度的间隔不符：%v", got)
	}
}

// ── Next：新卡首次评级 ──

func TestFsrsNextOnNewCard(t *testing.T) {
	f := NewFsrs(nil)
	now := time.Date(2026, 3, 10, 9, 0, 0, 0, time.Local)

	for _, rating := range []FsrsRating{RatingAgain, RatingHard, RatingGood, RatingEasy} {
		res := f.Next(&FsrsCard{State: StateNew}, now, rating)
		if res == nil {
			t.Fatalf("评级 %d 返回 nil", rating)
		}
		if res.Card.Reps != 1 {
			t.Fatalf("首次复习 reps = %d, want 1", res.Card.Reps)
		}
		if res.Card.Stability <= 0 {
			t.Fatalf("稳定度应大于 0，实际 %v", res.Card.Stability)
		}
		if res.Card.Difficulty < 1 || res.Card.Difficulty > 10 {
			t.Fatalf("难度越界：%v", res.Card.Difficulty)
		}
		if res.Card.LastReview == nil || !res.Card.LastReview.Equal(now) {
			t.Fatalf("LastReview 应为本次复习时间：%v", res.Card.LastReview)
		}
		if !res.Due.After(now) {
			t.Fatalf("到期时间应在复习之后：Due=%v now=%v", res.Due, now)
		}
		if res.Due != res.Card.Due {
			t.Fatal("结果里的 Due 应与卡片上的一致")
		}
	}
}

// TestFsrsRepeatedGoodReviewsGrowInterval 是 FSRS 的核心性质：
// 连续按 Good 复习（且每次都在到期日复习）时，稳定性与复习间隔应当单调递增。
func TestFsrsRepeatedGoodReviewsGrowInterval(t *testing.T) {
	f := NewFsrs(nil)
	now := time.Date(2026, 3, 10, 9, 0, 0, 0, time.Local)

	card := &FsrsCard{State: StateReview, Stability: 5, Difficulty: 5, LastReview: &now, Due: now}
	prevInterval, prevStability := 0, 0.0

	for i := 0; i < 6; i++ {
		res := f.Next(card, card.Due, RatingGood)
		if res.Interval <= prevInterval {
			t.Fatalf("第 %d 次复习间隔 %d 未比上次（%d）增长", i+1, res.Interval, prevInterval)
		}
		if res.Card.Stability <= prevStability {
			t.Fatalf("第 %d 次复习稳定度 %v 未比上次（%v）增长", i+1, res.Card.Stability, prevStability)
		}
		if res.Card.State != StateReview {
			t.Fatalf("第 %d 次复习后状态 = %d, want StateReview", i+1, res.Card.State)
		}
		prevInterval, prevStability = res.Interval, res.Card.Stability
		next := res.Card
		card = &next
	}
}

// TestFsrsAgainPenalty 复习卡「忘记」后应累计 lapse、稳定度下降，并按重学步骤重新排期。
// 默认重学步骤只有一个「1d」，达到整天的步骤会直接毕业回复习态。
func TestFsrsAgainPenalty(t *testing.T) {
	f := NewFsrs(nil)
	now := time.Date(2026, 3, 10, 9, 0, 0, 0, time.Local)
	last := now.AddDate(0, 0, -10)

	card := &FsrsCard{
		State: StateReview, Stability: 20, Difficulty: 5,
		LastReview: &last, Due: now, Lapses: 1, Reps: 3,
	}

	res := f.Next(card, now, RatingAgain)
	if res.Card.Lapses != 2 {
		t.Fatalf("lapses = %d, want 2", res.Card.Lapses)
	}
	if res.Card.Reps != 4 {
		t.Fatalf("reps = %d, want 4", res.Card.Reps)
	}
	if res.Card.Stability >= 20 {
		t.Fatalf("忘记后稳定度应下降，实际 %v", res.Card.Stability)
	}
	if res.Card.ElapsedDays != 10 {
		t.Fatalf("elapsed_days = %d, want 10", res.Card.ElapsedDays)
	}
	if res.Card.State != StateReview {
		t.Fatalf("1 天重学步骤应直接毕业回复习态，实际 %d", res.Card.State)
	}
	if !res.Due.After(now) || res.Interval < 1 {
		t.Fatalf("重学排期异常：Interval=%d Due=%v", res.Interval, res.Due)
	}
}

// TestFsrsAgainWithShortStepStaysInRelearning 分钟级重学步骤应把卡片留在重学队列里。
func TestFsrsAgainWithShortStepStaysInRelearning(t *testing.T) {
	params := DefaultFsrsParameters()
	params.RelearningSteps = []string{"10m"}
	f := NewFsrs(params)

	now := time.Date(2026, 3, 10, 9, 0, 0, 0, time.Local)
	card := &FsrsCard{State: StateReview, Stability: 20, Difficulty: 5, LastReview: &now, Due: now}

	res := f.Next(card, now, RatingAgain)
	if res.Card.State != StateRelearning {
		t.Fatalf("分钟级重学步骤应留在重学态，实际 %d", res.Card.State)
	}
	if res.Card.ScheduledDays != 0 {
		t.Fatalf("当天内的重学不应按整天计，实际 %d", res.Card.ScheduledDays)
	}
	if !res.Due.After(now) {
		t.Fatalf("10 分钟步骤的到期时间应为当天稍后，实际 %v", res.Due)
	}
	if res.Due.Sub(now) > 24*time.Hour {
		t.Fatalf("10 分钟步骤不应排到一天之后，实际 %v", res.Due)
	}
}

func TestFsrsRespectsMaximumInterval(t *testing.T) {
	params := DefaultFsrsParameters()
	params.MaximumInterval = 10
	f := NewFsrs(params)

	now := time.Date(2026, 3, 10, 9, 0, 0, 0, time.Local)
	last := now.AddDate(0, 0, -10)
	card := &FsrsCard{State: StateReview, Stability: 30000, Difficulty: 1, LastReview: &last, Due: now}

	for _, rating := range []FsrsRating{RatingHard, RatingGood, RatingEasy} {
		res := f.Next(card, now, rating)
		if res.Interval > 10 {
			t.Fatalf("评级 %d 的间隔 %d 超出 MaximumInterval=10", rating, res.Interval)
		}
		if res.Interval < 1 {
			t.Fatalf("评级 %d 的间隔 %d 小于 1 天", rating, res.Interval)
		}
	}
}

func TestFsrsIntervalOrderingAgainHardGoodEasy(t *testing.T) {
	f := NewFsrs(nil)
	now := time.Date(2026, 3, 10, 9, 0, 0, 0, time.Local)
	last := now.AddDate(0, 0, -5)
	base := FsrsCard{State: StateReview, Stability: 20, Difficulty: 5, LastReview: &last, Due: now}

	hard := f.Next(&base, now, RatingHard)
	good := f.Next(&base, now, RatingGood)
	easy := f.Next(&base, now, RatingEasy)

	if !(hard.Interval <= good.Interval && good.Interval <= easy.Interval) {
		t.Fatalf("复习态下应有 hard ≤ good ≤ easy，实际 %d / %d / %d",
			hard.Interval, good.Interval, easy.Interval)
	}
	if !(hard.Card.Stability <= good.Card.Stability && good.Card.Stability <= easy.Card.Stability) {
		t.Fatalf("稳定度顺序不符：%v / %v / %v",
			hard.Card.Stability, good.Card.Stability, easy.Card.Stability)
	}
}

// TestFsrsFuzzDeterministic FSRS 有随机抖动，但种子由卡片状态推导，
// 因此对同一张卡同一时刻的调度结果必须是可复现的（否则任务计划的重排会漂移）。
func TestFsrsFuzzDeterministic(t *testing.T) {
	params := DefaultFsrsParameters()
	params.EnableFuzz = true
	f := NewFsrs(params)

	now := time.Date(2026, 3, 10, 9, 0, 0, 0, time.Local)
	last := now.AddDate(0, 0, -20)
	newCard := func() *FsrsCard {
		return &FsrsCard{State: StateReview, Stability: 30, Difficulty: 5, LastReview: &last, Due: now}
	}

	first := f.Next(newCard(), now, RatingGood)
	second := f.Next(newCard(), now, RatingGood)
	if first.Interval != second.Interval || !first.Due.Equal(second.Due) {
		t.Fatalf("抖动结果不可复现：%d/%v 与 %d/%v", first.Interval, first.Due, second.Interval, second.Due)
	}

	// 抖动不能把间隔压到 1 天以下
	for i := 0; i < 20; i++ {
		res := f.Next(newCard(), now.AddDate(0, 0, i), RatingGood)
		if res.Interval < 1 {
			t.Fatalf("抖动后间隔 %d 小于 1 天", res.Interval)
		}
	}
}

func TestFsrsUnknownStateIsNil(t *testing.T) {
	f := NewFsrs(nil)
	now := time.Now()
	if res := f.Next(&FsrsCard{State: FsrsState(99)}, now, RatingGood); res != nil {
		t.Fatalf("未知状态应返回 nil，实际 %+v", res)
	}
}

func TestFsrsDefaultParametersSanity(t *testing.T) {
	p := DefaultFsrsParameters()
	if len(p.Weights) != 21 {
		t.Fatalf("权重数量 = %d, want 21（算法按下标取到 w20）", len(p.Weights))
	}
	if p.RequestRetention <= 0 || p.RequestRetention >= 1 {
		t.Fatalf("目标留存率应在 (0,1) 之间，实际 %v", p.RequestRetention)
	}
	if strings.Join(p.LearningSteps, ",") == "" {
		t.Fatal("默认学习步骤不应为空")
	}
}
