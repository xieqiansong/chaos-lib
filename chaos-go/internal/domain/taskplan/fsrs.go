package taskplan

import (
	"time"

	"github.com/open-spaced-repetition/go-fsrs"
)

// FSRS 间隔重复调度：算法本体交给官方实现 github.com/open-spaced-repetition/go-fsrs。

type FsrsRating = fsrs.Rating

const (
	RatingAgain = fsrs.Again
	RatingHard  = fsrs.Hard
	RatingGood  = fsrs.Good
	RatingEasy  = fsrs.Easy
)

var fsrsParams = fsrs.DefaultParam()

type FsrsCard = fsrs.Card

type FsrsSchedulingResult struct {
	Card fsrs.Card
	Due  time.Time
}

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

func NextReview(card FsrsCard, now time.Time, rating FsrsRating) FsrsSchedulingResult {
	info := fsrsParams.Repeat(card, now)[rating]
	return FsrsSchedulingResult{Card: info.Card, Due: info.Card.Due}
}

func (r FsrsSchedulingResult) ApplyToPlan(plan *TaskPlan, now time.Time) {
	plan.FsrsStability = r.Card.Stability
	plan.FsrsDifficulty = r.Card.Difficulty
	plan.FsrsReps = int(r.Card.Reps)
	plan.FsrsLapses = int(r.Card.Lapses)
	plan.FsrsState = int(r.Card.State)
	plan.FsrsLastReviewAt = &now
}
