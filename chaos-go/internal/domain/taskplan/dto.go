package taskplan

import "time"

// ── 请求契约 ────────────────────────────────────────────────────

// CreatePlanRequest 创建任务计划的请求体。
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

// UpdatePlanRequest 更新任务计划的请求体（nil 字段表示不改；ParentID 为 nil 表示置空）。
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

// ReviewRequest 提交复习结果的请求体。
type ReviewRequest struct {
	Rating int             `json:"rating"`
	Answer string          `json:"answer"`
	AI     *ReviewAIResult `json:"ai"`
}

// ReviewAIResult AI 评分结果：前端回传，亦随复习内容落库到 tasks.remark。
type ReviewAIResult struct {
	Points []struct {
		Text    string `json:"text"`
		Covered bool   `json:"covered"`
		Reason  string `json:"reason"`
	} `json:"points"`
	Coverage        int `json:"coverage"`
	SuggestedRating int `json:"suggestedRating"`
}

// ── 响应契约 ────────────────────────────────────────────────────

// PlanResponse 计划详情响应（创建 / 更新后的形态）。
type PlanResponse struct {
	ID        int             `json:"id"`
	ParentID  *int            `json:"parentId"`
	Name      string          `json:"name"`
	Status    TaskPlanStatus  `json:"status"`
	PlanType  TaskPlanType    `json:"planType"`
	CronExpr  *string         `json:"cronExpr"`
	OrderNum  int             `json:"orderNum"`
	Priority  int             `json:"priority"`
	Remark    *string         `json:"remark"`
	Link      *string         `json:"link"`
	CreatedAt time.Time       `json:"createdAt"`
	UpdatedAt time.Time       `json:"updatedAt"`
	FirstTask TaskView        `json:"firstTask,omitempty"`
}

// PlanStateResponse 计划状态变更响应（开启计划）。
type PlanStateResponse struct {
	ID        int                    `json:"id"`
	Name      string                 `json:"name"`
	Status    TaskPlanStatus         `json:"status"`
	PlanType  TaskPlanType           `json:"planType"`
	UpdatedAt time.Time              `json:"updatedAt"`
	FirstTask TaskView               `json:"firstTask,omitempty"`
}

// DeletePlanResponse 删除计划结果。
type DeletePlanResponse struct {
	Message          string `json:"message"`
	Cascade          bool   `json:"cascade"`
	DeletedPlanCount *int   `json:"deletedPlanCount,omitempty"`
}

// ReviewResponse 复习结果（含 FSRS 记忆参数与下次复习时间）。
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

// ReviewPoint AI 评分的单个关键点。
type ReviewPoint struct {
	Text    string `json:"text"`
	Covered bool   `json:"covered"`
	Reason  string `json:"reason"`
}

// ReviewScore AI 评分结果（也是 deepseek 返回体的解析目标）。
type ReviewScore struct {
	Points          []ReviewPoint `json:"points"`
	Coverage        int           `json:"coverage"`
	SuggestedRating int           `json:"suggestedRating"`
}

// PlanRaw 计划关联的原文内容（服务端代理拉取，规避浏览器跨域）。
type PlanRaw struct {
	RawLink string `json:"rawLink"`
	Content string `json:"content"`
}

// PostponeItemResult 批量延期单项结果。
type PostponeItemResult struct {
	ID     int    `json:"id"`
	Status string `json:"status"`
	Reason string `json:"reason,omitempty"`
}

// BatchPostponeResult 批量延期汇总。
type BatchPostponeResult struct {
	Postponed int                  `json:"postponed"`
	Skipped   int                  `json:"skipped"`
	Results   []PostponeItemResult `json:"results"`
}

// DailyPoint 单日统计点。
type DailyPoint struct {
	Date  string `json:"date"`
	Count int    `json:"count"`
}

// ContributionItem 贡献热力图的单个统计项。
type ContributionItem struct {
	ID    int          `json:"id"`
	Name  string       `json:"name"`
	Total int          `json:"total"`
	Days  []DailyPoint `json:"days"`
}

// ContributionStats 贡献热力图响应。
type ContributionStats struct {
	RootName string             `json:"rootName"`
	Start    string             `json:"start"`
	End      string             `json:"end"`
	Items    []ContributionItem `json:"items"`
}

// TaskView 任务视图：仅输出前端需要的字段，空值字段省略。
type TaskView map[string]interface{}

// taskToView 组装任务视图。
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
