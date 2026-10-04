package taskplan

import (
	"testing"
	"time"
)

// 冒烟测试：FSRS 换用官方实现后，验证调度行为仍然合理（不校验具体数值，只校验方向性）。
func TestNextReview(t *testing.T) {
	now := time.Date(2026, 10, 4, 12, 0, 0, 0, time.UTC)

	// 新卡片：四档评分都要给出晚于 now 的到期时间，且 Easy 不早于 Good、Good 不早于 Hard
	newDues := map[FsrsRating]time.Time{}
	for _, r := range []FsrsRating{RatingAgain, RatingHard, RatingGood, RatingEasy} {
		card := CardFromPlan(&TaskPlan{})
		if card.State != 0 {
			t.Fatalf("无复习记录的卡片应视为新卡片，实际 state=%d", card.State)
		}
		res := NextReview(card, now, r)
		if !res.Due.After(now) {
			t.Fatalf("rating=%d 到期时间 %v 应晚于 %v", r, res.Due, now)
		}
		newDues[r] = res.Due
		t.Logf("新卡片 rating=%d → state=%d stability=%.4f difficulty=%.4f after=%v",
			r, res.Card.State, res.Card.Stability, res.Card.Difficulty, res.Due.Sub(now))
	}
	if newDues[RatingEasy].Before(newDues[RatingGood]) {
		t.Fatalf("Easy 的到期时间不应早于 Good")
	}
	if newDues[RatingGood].Before(newDues[RatingHard]) {
		t.Fatalf("Good 的到期时间不应早于 Hard")
	}

	// 复习态卡片：5 天后按 Good 复习，稳定性应增长、复习次数 +1
	last := now.AddDate(0, 0, -5)
	plan := &TaskPlan{
		FsrsStability:    5,
		FsrsDifficulty:   5,
		FsrsReps:         3,
		FsrsLapses:       0,
		FsrsState:        2, // Review
		FsrsLastReviewAt: &last,
	}
	res := NextReview(CardFromPlan(plan), now, RatingGood)
	res.ApplyToPlan(plan, now)
	if plan.FsrsStability <= 5 {
		t.Fatalf("Good 复习后 stability 应增长，实际 %.4f（原 5）", plan.FsrsStability)
	}
	if plan.FsrsReps != 4 {
		t.Fatalf("复习次数应 +1，实际 %d", plan.FsrsReps)
	}
	if plan.FsrsLastReviewAt == nil || !plan.FsrsLastReviewAt.Equal(now) {
		t.Fatalf("应回写本次复习时间，实际 %v", plan.FsrsLastReviewAt)
	}
	t.Logf("复习态 → stability=%.4f difficulty=%.4f reps=%d state=%d after=%v",
		plan.FsrsStability, plan.FsrsDifficulty, plan.FsrsReps, plan.FsrsState, res.Due.Sub(now))

	// Again 应触发 lapse 并进入重新学习
	plan2 := &TaskPlan{FsrsStability: 10, FsrsDifficulty: 5, FsrsReps: 5, FsrsState: 2, FsrsLastReviewAt: &last}
	res2 := NextReview(CardFromPlan(plan2), now, RatingAgain)
	res2.ApplyToPlan(plan2, now)
	if plan2.FsrsLapses != 1 {
		t.Fatalf("Again 应使 lapses +1，实际 %d", plan2.FsrsLapses)
	}
	t.Logf("Again → lapses=%d state=%d stability=%.4f after=%v",
		plan2.FsrsLapses, plan2.FsrsState, plan2.FsrsStability, res2.Due.Sub(now))
}
