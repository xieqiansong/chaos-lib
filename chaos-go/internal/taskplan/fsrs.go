package taskplan

import (
	"math"
	"math/rand/v2"
	"strconv"
	"time"
)

const (
	fsrsSMin = 0.001
	fsrsSMax = 36500
)

type FsrsState int

const (
	StateNew        FsrsState = 0
	StateLearning   FsrsState = 1
	StateReview     FsrsState = 2
	StateRelearning FsrsState = 3
)

type FsrsRating int

const (
	RatingAgain FsrsRating = 1
	RatingHard  FsrsRating = 2
	RatingGood  FsrsRating = 3
	RatingEasy  FsrsRating = 4
)

type FsrsCard struct {
	Due           time.Time
	Stability     float64
	Difficulty    float64
	ElapsedDays   int
	ScheduledDays int
	Reps          int
	Lapses        int
	LearningSteps int
	State         FsrsState
	LastReview    *time.Time
}

type FsrsParameters struct {
	RequestRetention float64
	MaximumInterval  int
	Weights          []float64
	EnableFuzz       bool
	EnableShortTerm  bool
	LearningSteps    []string
	RelearningSteps  []string
}

type FsrsSchedulingResult struct {
	Card     FsrsCard
	Due      time.Time
	Interval int
}

type Fsrs struct {
	params *FsrsParameters
}

func DefaultFsrsParameters() *FsrsParameters {
	return &FsrsParameters{
		RequestRetention: 0.95,
		MaximumInterval:  365,
		Weights: []float64{
			0.212, 1.2931, 2.3065, 8.2956, 6.4133, 0.8334, 3.0194, 0.001, 1.8722, 0.1666,
			0.796, 1.4835, 0.0614, 0.2629, 1.6483, 0.6014, 1.8729, 0.5425, 0.0912, 0.0658, 0.1542,
		},
		EnableFuzz:      false,
		EnableShortTerm: true,
		LearningSteps:   []string{"1d"},
		RelearningSteps: []string{"1d"},
	}
}

func NewFsrs(params *FsrsParameters) *Fsrs {
	if params == nil {
		params = DefaultFsrsParameters()
	}
	return &Fsrs{params: params}
}

func (f *Fsrs) computeDecayFactor() (decay, factor float64) {
	decay = -f.params.Weights[20]
	factor = math.Exp(math.Pow(decay, -1)*math.Log(0.9)) - 1
	return
}

func (f *Fsrs) calculateIntervalModifier() float64 {
	decay, factor := f.computeDecayFactor()
	return (math.Pow(f.params.RequestRetention, 1/decay) - 1) / factor
}

func (f *Fsrs) initStability(grade FsrsRating) float64 {
	return math.Max(f.params.Weights[grade-1], 0.1)
}

func (f *Fsrs) initDifficulty(grade FsrsRating) float64 {
	return f.params.Weights[4] - math.Exp(float64(grade-1)*f.params.Weights[5]) + 1
}

func (f *Fsrs) forgettingCurve(elapsedDays int, stability float64) float64 {
	decay, factor := f.computeDecayFactor()
	stability = math.Max(stability, fsrsSMin)
	return math.Pow(1+factor*float64(elapsedDays)/stability, decay)
}

func (f *Fsrs) linearDamping(deltaD, oldD float64) float64 {
	return deltaD * (10 - oldD) / 9
}

func (f *Fsrs) meanReversion(init, current float64) float64 {
	return f.params.Weights[7]*init + (1-f.params.Weights[7])*current
}

func (f *Fsrs) nextDifficulty(d float64, grade FsrsRating) float64 {
	deltaD := -f.params.Weights[6] * (float64(grade) - 3)
	nextD := d + f.linearDamping(deltaD, d)
	return clamp(f.meanReversion(f.initDifficulty(RatingEasy), nextD), 1, 10)
}

func (f *Fsrs) nextRecallStability(d, s, r float64, grade FsrsRating) float64 {
	s = math.Max(s, fsrsSMin)
	hardPenalty := 1.0
	if grade == RatingHard {
		hardPenalty = f.params.Weights[15]
	}
	easyBonus := 1.0
	if grade == RatingEasy {
		easyBonus = f.params.Weights[16]
	}
	return clamp(s*(1+math.Exp(f.params.Weights[8])*(11-d)*math.Pow(s, -f.params.Weights[9])*(math.Exp((1-r)*f.params.Weights[10])-1)*hardPenalty*easyBonus), fsrsSMin, fsrsSMax)
}

func (f *Fsrs) nextForgetStability(d, s, r float64) float64 {
	return clamp(f.params.Weights[11]*math.Pow(d, -f.params.Weights[12])*(math.Pow(s+1, f.params.Weights[13])-1)*math.Exp((1-r)*f.params.Weights[14]), fsrsSMin, fsrsSMax)
}

func (f *Fsrs) nextShortTermStability(s float64, grade FsrsRating) float64 {
	s = math.Max(s, fsrsSMin)
	sinc := math.Pow(s, -f.params.Weights[19]) * math.Exp(f.params.Weights[17]*(float64(grade)-3+f.params.Weights[18]))
	maskedSinc := sinc
	if grade >= RatingGood {
		maskedSinc = math.Max(sinc, 1)
	}
	return clamp(s*maskedSinc, fsrsSMin, fsrsSMax)
}

func (f *Fsrs) nextIntervalRaw(s float64) int {
	intervalModifier := f.calculateIntervalModifier()
	newInterval := math.Min(math.Max(1, math.Round(s*intervalModifier)), float64(f.params.MaximumInterval))
	return int(newInterval)
}

func (f *Fsrs) nextInterval(s float64, elapsedDays int, rng *rand.Rand) int {
	newInterval := f.nextIntervalRaw(s)
	return f.applyFuzz(newInterval, elapsedDays, rng)
}

func (f *Fsrs) applyFuzz(ivl, elapsedDays int, rng *rand.Rand) int {
	if !f.params.EnableFuzz || float64(ivl) < 2.5 {
		return ivl
	}
	fuzzFactor := rng.Float64()
	minIvl, maxIvl := f.getFuzzRange(ivl, elapsedDays)
	return int(math.Floor(fuzzFactor*float64(maxIvl-minIvl+1) + float64(minIvl)))
}

func (f *Fsrs) getFuzzRange(ivl, elapsedDays int) (int, int) {
	fuzzRanges := []struct{ start, end, factor float64 }{
		{2.5, 7, 0.15},
		{7, 20, 0.1},
		{20, math.Inf(1), 0.05},
	}
	delta := 1.0
	for _, rng := range fuzzRanges {
		delta += rng.factor * math.Max(math.Min(float64(ivl), rng.end)-rng.start, 0)
	}
	ivlF := math.Min(float64(ivl), float64(f.params.MaximumInterval))
	minIvl := int(math.Max(2, math.Round(ivlF-delta)))
	maxIvl := int(math.Min(math.Round(ivlF+delta), float64(f.params.MaximumInterval)))
	if ivl > elapsedDays {
		minIvl = maxInt(minIvl, elapsedDays+1)
	}
	minIvl = minInt(minIvl, maxIvl)
	return minIvl, maxIvl
}

type learningStepResult struct {
	scheduledMinutes int
	nextStep         int
}

func basicLearningStepsStrategy(params *FsrsParameters, state FsrsState, curStep int) map[FsrsRating]learningStepResult {
	var steps []string
	if state == StateRelearning || state == StateReview {
		steps = params.RelearningSteps
	} else {
		steps = params.LearningSteps
	}
	stepsLength := len(steps)
	if stepsLength == 0 || curStep >= stepsLength {
		return nil
	}
	firstStep := convertStepToMinutes(steps[0])
	getAgainInterval := firstStep
	getHardInterval := 0
	if stepsLength == 1 {
		getHardInterval = int(math.Round(float64(firstStep) * 1.5))
	} else {
		getHardInterval = int(math.Round(float64(firstStep+convertStepToMinutes(steps[1])) / 2))
	}
	result := make(map[FsrsRating]learningStepResult)
	if state == StateReview {
		stepInfo := steps[maxInt(0, curStep)]
		result[RatingAgain] = learningStepResult{convertStepToMinutes(stepInfo), 0}
		return result
	}
	result[RatingAgain] = learningStepResult{getAgainInterval, 0}
	result[RatingHard] = learningStepResult{getHardInterval, curStep}
	if curStep+1 < stepsLength {
		nextMin := convertStepToMinutes(steps[curStep+1])
		result[RatingGood] = learningStepResult{int(math.Round(float64(nextMin))), curStep + 1}
	}
	return result
}

func (f *Fsrs) Next(card *FsrsCard, now time.Time, rating FsrsRating) *FsrsSchedulingResult {
	elapsedDays := 0
	if card.State != StateNew && card.LastReview != nil {
		elapsedDays = dateDiffInDays(*card.LastReview, now)
	}
	lastReview := now
	reps := card.Reps + 1

	seed := now.UnixNano() + int64(reps) + int64((card.Difficulty*card.Stability)*1000)
	rng := rand.New(rand.NewPCG(uint64(seed), uint64(seed>>32)))

	switch card.State {
	case StateNew:
		return f.newState(card, now, rating, elapsedDays, reps, lastReview, rng)
	case StateLearning, StateRelearning:
		return f.learningState(card, now, rating, elapsedDays, reps, lastReview, rng)
	case StateReview:
		return f.reviewState(card, now, rating, elapsedDays, reps, lastReview, rng)
	}
	return nil
}

func (f *Fsrs) copyCard(card *FsrsCard, elapsedDays, reps int, lastReview time.Time) FsrsCard {
	return FsrsCard{
		Stability:     card.Stability,
		Difficulty:    card.Difficulty,
		ElapsedDays:   elapsedDays,
		ScheduledDays: 0,
		Reps:          reps,
		Lapses:        card.Lapses,
		LearningSteps: card.LearningSteps,
		State:         card.State,
		LastReview:    &lastReview,
	}
}

func (f *Fsrs) newState(card *FsrsCard, now time.Time, rating FsrsRating, elapsedDays, reps int, lastReview time.Time, rng *rand.Rand) *FsrsSchedulingResult {
	next := f.copyCard(card, elapsedDays, reps, lastReview)
	next.Difficulty = clamp(f.initDifficulty(rating), 1, 10)
	next.Stability = f.initStability(rating)
	f.applyLearningSteps(&next, card, now, rating, StateLearning, rng)
	return &FsrsSchedulingResult{Card: next, Due: next.Due, Interval: next.ScheduledDays}
}

func (f *Fsrs) learningState(card *FsrsCard, now time.Time, rating FsrsRating, elapsedDays, reps int, lastReview time.Time, rng *rand.Rand) *FsrsSchedulingResult {
	next := f.copyCard(card, elapsedDays, reps, lastReview)
	next.Difficulty = f.nextDifficulty(card.Difficulty, rating)
	next.Stability = f.nextShortTermStability(card.Stability, rating)
	f.applyLearningSteps(&next, card, now, rating, card.State, rng)
	return &FsrsSchedulingResult{Card: next, Due: next.Due, Interval: next.ScheduledDays}
}

func (f *Fsrs) reviewState(card *FsrsCard, now time.Time, rating FsrsRating, elapsedDays, reps int, lastReview time.Time, rng *rand.Rand) *FsrsSchedulingResult {
	d := card.Difficulty
	s := card.Stability
	r := f.forgettingCurve(elapsedDays, s)

	nextAgain := f.copyCard(card, elapsedDays, reps, lastReview)
	nextHard := f.copyCard(card, elapsedDays, reps, lastReview)
	nextGood := f.copyCard(card, elapsedDays, reps, lastReview)
	nextEasy := f.copyCard(card, elapsedDays, reps, lastReview)

	nextAgain.Difficulty = f.nextDifficulty(d, RatingAgain)
	nextHard.Difficulty = f.nextDifficulty(d, RatingHard)
	nextGood.Difficulty = f.nextDifficulty(d, RatingGood)
	nextEasy.Difficulty = f.nextDifficulty(d, RatingEasy)

	nextSMin := s / math.Exp(f.params.Weights[17]*f.params.Weights[18])
	sAfterFail := f.nextForgetStability(d, s, r)
	nextAgain.Stability = clamp(nextSMin, fsrsSMin, sAfterFail)
	nextHard.Stability = f.nextRecallStability(d, s, r, RatingHard)
	nextGood.Stability = f.nextRecallStability(d, s, r, RatingGood)
	nextEasy.Stability = f.nextRecallStability(d, s, r, RatingEasy)

	hardInterval := f.nextInterval(nextHard.Stability, elapsedDays, rng)
	goodInterval := f.nextInterval(nextGood.Stability, elapsedDays, rng)
	hardInterval = minInt(hardInterval, goodInterval)
	goodInterval = minInt(maxInt(goodInterval, hardInterval+1), f.params.MaximumInterval)
	easyInterval := minInt(maxInt(f.nextInterval(nextEasy.Stability, elapsedDays, rng), goodInterval+1), f.params.MaximumInterval)

	nextHard.ScheduledDays = hardInterval
	nextHard.Due = dateScheduler(now, hardInterval, true)
	nextGood.ScheduledDays = goodInterval
	nextGood.Due = dateScheduler(now, goodInterval, true)
	nextEasy.ScheduledDays = easyInterval
	nextEasy.Due = dateScheduler(now, easyInterval, true)

	nextHard.State = StateReview
	nextHard.LearningSteps = 0
	nextGood.State = StateReview
	nextGood.LearningSteps = 0
	nextEasy.State = StateReview
	nextEasy.LearningSteps = 0

	f.applyLearningSteps(&nextAgain, card, now, RatingAgain, StateRelearning, rng)
	nextAgain.Lapses++

	var resultCard FsrsCard
	switch rating {
	case RatingAgain:
		resultCard = nextAgain
	case RatingHard:
		resultCard = nextHard
	case RatingGood:
		resultCard = nextGood
	case RatingEasy:
		resultCard = nextEasy
	}

	return &FsrsSchedulingResult{Card: resultCard, Due: resultCard.Due, Interval: resultCard.ScheduledDays}
}

func (f *Fsrs) applyLearningSteps(next *FsrsCard, card *FsrsCard, now time.Time, rating FsrsRating, toState FsrsState, rng *rand.Rand) {
	scheduledMinutes, nextSteps := f.getLearningInfo(card, rating)
	if scheduledMinutes > 0 && scheduledMinutes < 1440 {
		next.LearningSteps = nextSteps
		next.ScheduledDays = 0
		next.State = toState
		next.Due = dateScheduler(now, scheduledMinutes, false)
	} else {
		next.State = StateReview
		if scheduledMinutes >= 1440 {
			next.LearningSteps = nextSteps
			next.Due = dateScheduler(now, scheduledMinutes, false)
			next.ScheduledDays = scheduledMinutes / 1440
		} else {
			next.LearningSteps = 0
			interval := f.nextInterval(next.Stability, next.ElapsedDays, rng)
			next.ScheduledDays = interval
			next.Due = dateScheduler(now, interval, true)
		}
	}
}

func (f *Fsrs) getLearningInfo(card *FsrsCard, rating FsrsRating) (scheduledMinutes, nextSteps int) {
	if card.LearningSteps < 0 {
		card.LearningSteps = 0
	}
	step := card.LearningSteps
	if card.State == StateLearning && rating != RatingAgain && rating != RatingHard {
		step = card.LearningSteps + 1
	}
	stepsStrategy := basicLearningStepsStrategy(f.params, card.State, step)
	if result, ok := stepsStrategy[rating]; ok {
		scheduledMinutes = result.scheduledMinutes
		nextSteps = result.nextStep
	}
	if scheduledMinutes < 0 {
		scheduledMinutes = 0
	}
	if nextSteps < 0 {
		nextSteps = 0
	}
	return
}

func clamp(value, min, max float64) float64 {
	return math.Min(math.Max(value, min), max)
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func dateDiffInDays(a, b time.Time) int {
	aLocal := time.Date(a.Year(), a.Month(), a.Day(), 0, 0, 0, 0, time.Local)
	bLocal := time.Date(b.Year(), b.Month(), b.Day(), 0, 0, 0, 0, time.Local)
	return int(bLocal.Sub(aLocal).Hours() / 24)
}

func dateScheduler(now time.Time, t int, isDay bool) time.Time {
	if isDay {
		return now.Add(time.Duration(t) * 24 * time.Hour)
	}
	return now.Add(time.Duration(t) * time.Minute)
}

func convertStepToMinutes(step string) int {
	if len(step) < 2 {
		return 0
	}
	unit := step[len(step)-1]
	value, err := strconv.Atoi(step[:len(step)-1])
	if err != nil || value < 0 {
		return 0
	}
	switch unit {
	case 'm':
		return value
	case 'h':
		return value * 60
	case 'd':
		return value * 1440
	default:
		return 0
	}
}
