package taskplan

import "time"

type CreatePlanRequest struct {
	ParentID  *int
	Name      string
	PlanType  TaskPlanType
	CronExpr  *string
	OrderNum  *int
	Priority  *int
	Remark    *string
	Link      *string
	StartedAt *time.Time
}

type UpdatePlanRequest struct {
	Name     *string
	ParentID *int
	PlanType TaskPlanType
	OrderNum *int
	Priority *int
	Remark   *string
	Link     *string
	CronExpr *string
}

type ReviewRequest struct {
	Rating int             `json:"rating"`
	Answer string          `json:"answer"`
	AI     *ReviewAIResult `json:"ai"`
}

type ReviewAIResult struct {
	Points []struct {
		Text    string `json:"text"`
		Covered bool   `json:"covered"`
		Reason  string `json:"reason"`
	} `json:"points"`
	Coverage        int `json:"coverage"`
	SuggestedRating int `json:"suggestedRating"`
}

type PlanResponse struct {
	ID        int            `json:"id"`
	ParentID  *int           `json:"parentId"`
	Name      string         `json:"name"`
	Status    TaskPlanStatus `json:"status"`
	PlanType  TaskPlanType   `json:"planType"`
	CronExpr  *string        `json:"cronExpr"`
	OrderNum  int            `json:"orderNum"`
	Priority  int            `json:"priority"`
	Remark    *string        `json:"remark"`
	Link      *string        `json:"link"`
	CreatedAt time.Time      `json:"createdAt"`
	UpdatedAt time.Time      `json:"updatedAt"`
	FirstTask TaskView       `json:"firstTask,omitempty"`
}

type PlanStateResponse struct {
	ID        int            `json:"id"`
	Name      string         `json:"name"`
	Status    TaskPlanStatus `json:"status"`
	PlanType  TaskPlanType   `json:"planType"`
	UpdatedAt time.Time      `json:"updatedAt"`
	FirstTask TaskView       `json:"firstTask,omitempty"`
}

type DeletePlanResponse struct {
	Message          string `json:"message"`
	Cascade          bool   `json:"cascade"`
	DeletedPlanCount *int   `json:"deletedPlanCount,omitempty"`
}

type ReviewResponse struct {
	PlanID         int        `json:"planId"`
	Name           string     `json:"name"`
	FsrsState      int        `json:"fsrsState"`
	FsrsStability  float64    `json:"fsrsStability"`
	FsrsDifficulty float64    `json:"fsrsDifficulty"`
	FsrsReps       int        `json:"fsrsReps"`
	FsrsLapses     int        `json:"fsrsLapses"`
	NextReviewAt   *time.Time `json:"nextReviewAt,omitempty"`
}

type ReviewPoint struct {
	Text    string `json:"text"`
	Covered bool   `json:"covered"`
	Reason  string `json:"reason"`
}

type ReviewScore struct {
	Points          []ReviewPoint `json:"points"`
	Coverage        int           `json:"coverage"`
	SuggestedRating int           `json:"suggestedRating"`
}

type PlanRaw struct {
	RawLink string `json:"rawLink"`
	Content string `json:"content"`
}

type PostponeItemResult struct {
	ID     int    `json:"id"`
	Status string `json:"status"`
	Reason string `json:"reason,omitempty"`
}

type BatchPostponeResult struct {
	Postponed int                  `json:"postponed"`
	Skipped   int                  `json:"skipped"`
	Results   []PostponeItemResult `json:"results"`
}

type DailyPoint struct {
	Date  string `json:"date"`
	Count int    `json:"count"`
}

type ContributionItem struct {
	ID    int          `json:"id"`
	Name  string       `json:"name"`
	Total int          `json:"total"`
	Days  []DailyPoint `json:"days"`
}

type ContributionStats struct {
	RootName string             `json:"rootName"`
	Start    string             `json:"start"`
	End      string             `json:"end"`
	Items    []ContributionItem `json:"items"`
}

type TaskView map[string]interface{}

func taskToView(t Task) TaskView {
	view := TaskView{
		"id":        t.ID,
		"planId":    t.PlanID,
		"status":    t.Status,
		"createdAt": t.CreatedAt,
	}
	if t.StartedAt != nil {
		view["startedAt"] = t.StartedAt
	}
	if t.CompletedAt != nil {
		view["completedAt"] = t.CompletedAt
	}
	if t.Deadline != nil {
		view["deadline"] = t.Deadline
	}
	if t.Remark != nil {
		view["remark"] = t.Remark
	}
	return view
}
