package taskplan

import (
	"time"

	"github.com/open-spaced-repetition/go-fsrs"
)

// FSRS 间隔重复调度：算法本体交给官方实现 github.com/open-spaced-repetition/go-fsrs。
//
// 本文件不再实现调度算法，只承担两件事：
//  1. 持有调度参数（官方默认权重，升级算法版本只需换库）；
//  2. 在「任务计划的记忆参数」与官方 Card 之间转换，屏蔽字段类型差异
//     （官方用 uint64 计数、值类型 LastReview，本包存 int 与 *time.Time）。
//
// 业务层（service.go）因此不感知库的类型细节，也不必再维护一套评分 / 状态枚举。

// FsrsRating 复习评分，沿用官方枚举。
type FsrsRating = fsrs.Rating

const (
	RatingAgain = fsrs.Again
	RatingHard  = fsrs.Hard
	RatingGood  = fsrs.Good
	RatingEasy  = fsrs.Easy
)

// fsrsParams 全局调度参数（官方默认权重）。
var fsrsParams = fsrs.DefaultParam()

// FsrsCard 一次复习的输入 / 输出卡片，即官方卡片类型。
type FsrsCard = fsrs.Card

// FsrsSchedulingResult 一次复习的调度结果：推进后的卡片与下次到期时间。
type FsrsSchedulingResult struct {
	Card fsrs.Card
	Due  time.Time
}

// CardFromPlan 用任务计划上的记忆参数还原一张 FSRS 卡片。
// 尚无复习记录时按新卡片处理：官方库以零值时间算 elapsedDays 会得出失真的巨大间隔。
func CardFromPlan(plan *TaskPlan) FsrsCard {
	card := fsrs.Card{
		Stability:  plan.FsrsStability,
		Difficulty: plan.FsrsDifficulty,
		Reps:       uint64(max(plan.FsrsReps, 0)),
		Lapses:     uint64(max(plan.FsrsLapses, 0)),
		State:      fsrs.State(plan.FsrsState),
	}
	if plan.FsrsLastReviewAt != nil {
		card.LastReview = *plan.FsrsLastReviewAt
	} else {
		card.State = fsrs.New
	}
	return card
}

// NextReview 按评分推进一次复习，返回推进后的卡片与下次到期时间。
func NextReview(card FsrsCard, now time.Time, rating FsrsRating) FsrsSchedulingResult {
	info := fsrsParams.Repeat(card, now)[rating]
	return FsrsSchedulingResult{Card: info.Card, Due: info.Card.Due}
}

// ApplyToPlan 把一次复习的结果写回任务计划的记忆参数。
// FsrsLearningSteps 是早期自研实现的遗留字段，官方卡片无对应概念，不再回写。
func (r FsrsSchedulingResult) ApplyToPlan(plan *TaskPlan, now time.Time) {
	plan.FsrsStability = r.Card.Stability
	plan.FsrsDifficulty = r.Card.Difficulty
	plan.FsrsReps = int(r.Card.Reps)
	plan.FsrsLapses = int(r.Card.Lapses)
	plan.FsrsState = int(r.Card.State)
	plan.FsrsLastReviewAt = &now
}
