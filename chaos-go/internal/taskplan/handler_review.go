package taskplan

import (
	"chaos-go/config"
	"chaos-go/internal/deepseek"
	renv "chaos-go/internal/resp"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

// GetTaskPlanRaw 读取计划关联的 raw_link 原文内容（服务端代理拉取，规避浏览器跨域）。
func GetTaskPlanRaw(c *gin.Context) {
	id, ok := getPlanID(c)
	if !ok {
		return
	}

	var plan TaskPlan
	if err := config.GetDB().Where("id = ? AND is_deleted = ?", id, false).First(&plan).Error; err != nil {
		renv.Error(c, http.StatusNotFound, "任务计划不存在")
		return
	}

	if plan.RawLink == nil || *plan.RawLink == "" {
		renv.Error(c, http.StatusNotFound, "该计划没有 raw_link，无法获取原文")
		return
	}

	content, err := fetchRawContent(*plan.RawLink)
	if err != nil {
		renv.Error(c, http.StatusBadGateway, "获取原文失败: " + err.Error())
		return
	}

	renv.Success(c, gin.H{
		"rawLink": *plan.RawLink,
		"content": content,
	})
}

// fetchRawContent 拉取给定 URL 的文本内容（用于代理 chaos-knots 的 raw 接口）。
func fetchRawContent(rawURL string) (string, error) {
	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Get(rawURL)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("状态码 %d", resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	return string(body), nil
}

// ReviewTaskPlan 主动回忆式复习：先在前端隐藏原文、用户填写回忆，再提交评分。
// 评分驱动 FSRS 调度（interval 类型），并把用户的回忆内容写入任务 remark 以便后续对比。
func ReviewTaskPlan(c *gin.Context) {
	id, ok := getPlanID(c)
	if !ok {
		return
	}

	var req struct {
		Rating int             `json:"rating"`
		Answer string          `json:"answer"`
		AI     *reviewAIResult `json:"ai"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		renv.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	if req.Rating < int(RatingAgain) || req.Rating > int(RatingEasy) {
		renv.Error(c, http.StatusBadRequest, "无效的评分，有效值: 1=Again, 2=Hard, 3=Good, 4=Easy")
		return
	}

	var plan TaskPlan
	if err := config.GetDB().Where("id = ? AND is_deleted = ?", id, false).First(&plan).Error; err != nil {
		renv.Error(c, http.StatusNotFound, "任务计划不存在")
		return
	}
	if plan.Status == TaskPlanStatusCompleted || plan.Status == TaskPlanStatusArchived {
		renv.Error(c, http.StatusBadRequest, "已完成/已归档的计划不可复习")
		return
	}

	db := config.GetDB()
	var task Task
	err := db.Where("plan_id = ? AND status = ? AND is_deleted = ?", id, TaskStatusActive, false).First(&task).Error
	if err != nil {
		// 没有进行中的任务：按需生成一条（等价于"现在就复习一次"）
		if plan.Status != TaskPlanStatusStarted && plan.Status != TaskPlanStatusCreated {
			renv.Error(c, http.StatusBadRequest, "当前状态无法发起复习")
			return
		}
		generated, gerr := generateTask(&plan, time.Now(), nil)
		if gerr != nil {
			renv.Error(c, http.StatusInternalServerError, "生成复习任务失败: " + gerr.Error())
			return
		}
		task = *generated
	}

	// 持久化用户的回忆内容 + AI 评分结果（JSON 格式写入 tasks.remark）
	remarkObj := map[string]interface{}{
		"answer": req.Answer,
	}
	if req.AI != nil {
		remarkObj["ai"] = req.AI
	}
	if remarkBytes, merr := json.Marshal(remarkObj); merr == nil {
		remarkStr := string(remarkBytes)
		task.Remark = &remarkStr
	} else {
		ans := req.Answer
		task.Remark = &ans
	}

	var rating *FsrsRating
	if plan.PlanType == TaskPlanTypeInterval {
		r := FsrsRating(req.Rating)
		rating = &r
	}

	nextTask, ferr := finishTask(&task, &plan, rating)
	if ferr != nil {
		renv.Error(c, http.StatusInternalServerError, "复习失败: " + ferr.Error())
		return
	}

	resp := gin.H{
		"planId":         plan.ID,
		"name":           plan.Name,
		"fsrsState":      plan.FsrsState,
		"fsrsStability":  plan.FsrsStability,
		"fsrsDifficulty": plan.FsrsDifficulty,
		"fsrsReps":       plan.FsrsReps,
		"fsrsLapses":     plan.FsrsLapses,
	}
	if nextTask != nil && nextTask.StartedAt != nil {
		resp["nextReviewAt"] = nextTask.StartedAt
	}

	renv.Success(c, resp)
}

// reviewScoreSystemPrompt 是固定的角色与评分标准提示（每次调用相同，省 token）。
const reviewScoreSystemPrompt = `你是一位严谨的「主动回忆(active recall)」学习评分员。用户会提供两段内容：
1) 【原文】：需要学习/记忆的材料。
2) 【我的回忆】：用户在未看原文情况下回忆写下的内容（关键词、要点或短句）。

你的任务：
- 从【原文】中提炼 3~5 个「必须记住的关键点」，覆盖核心概念、方法、结论或易错点；不要超过 5 个，尽量精炼。
- 逐条判断【我的回忆】是否覆盖了该关键点（同义、要点到位即算命中，不要求字字对应）。
- 计算覆盖度 coverage = 命中条数 / 总条数 * 100（整数）。
- 给出建议评分 suggestedRating（1~4 整数）：
  1=Again（几乎没想起来，覆盖度<40% 或关键结论错误）
  2=Hard（想起一部分、有明显遗漏，覆盖度 40%~69%）
  3=Good（基本完整、少量遗漏，覆盖度 70%~89%）
  4=Easy（完整且准确，覆盖度>=90%）。

只输出如下 JSON（不要任何额外文字、不要 markdown 代码块）：
{
  "points": [{"text":"关键点表述","covered":true,"reason":"一句简短说明命中/遗漏原因"}],
  "coverage": 80,
  "suggestedRating": 3
}`

// reviewAIResult 是前端回传的 AI 评分结果，亦用于落库到 tasks.remark。
type reviewAIResult struct {
	Points []struct {
		Text    string `json:"text"`
		Covered bool   `json:"covered"`
		Reason  string `json:"reason"`
	} `json:"points"`
	Coverage        int `json:"coverage"`
	SuggestedRating int `json:"suggestedRating"`
}

// AiReviewScore 用 DeepSeek 对「回忆答案」相对「原文」做覆盖度评分。
// 入参：{ "original": 原文文本, "answer": 用户回忆文本 }（原文由前端已拉取的内容传入，密钥不出服务端）。
// 出参：{ "points":[{text,covered,reason}], "coverage":int, "suggestedRating":1..4 }。
func AiReviewScore(c *gin.Context) {
	var req struct {
		Original string `json:"original"`
		Answer   string `json:"answer"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		renv.Error(c, http.StatusBadRequest, "请求格式错误: " + err.Error())
		return
	}
	original := strings.TrimSpace(req.Original)
	answer := strings.TrimSpace(req.Answer)
	if original == "" {
		renv.Error(c, http.StatusBadRequest, "缺少原文内容，无法评分")
		return
	}

	// 控制 token 规模，避免长文拖累延迟与费用
	if len(original) > 4096 {
		original = original[:4096] + "\n…（原文过长已截断）"
	}
	if len(answer) > 1024 {
		answer = answer[:1024] + "\n…（答案过长已截断）"
	}

	userMsg := "【原文】\n" + original + "\n\n【我的回忆】\n" + answer

	raw, err := deepseek.Chat(reviewScoreSystemPrompt, userMsg)
	if err != nil {
		renv.Error(c, http.StatusBadGateway, "AI 评分失败: " + err.Error())
		return
	}

	var result struct {
		Points []struct {
			Text    string `json:"text"`
			Covered bool   `json:"covered"`
			Reason  string `json:"reason"`
		} `json:"points"`
		Coverage        int `json:"coverage"`
		SuggestedRating int `json:"suggestedRating"`
	}
	if err := json.Unmarshal([]byte(raw), &result); err != nil {
		renv.Error(c, http.StatusBadGateway, "AI 返回解析失败")
		return
	}

	if result.Coverage < 0 {
		result.Coverage = 0
	}
	if result.Coverage > 100 {
		result.Coverage = 100
	}
	// 规整建议档位到 FSRS 四档；异常时用覆盖度兜底映射
	if result.SuggestedRating < 1 || result.SuggestedRating > 4 {
		switch {
		case result.Coverage >= 90:
			result.SuggestedRating = 4
		case result.Coverage >= 70:
			result.SuggestedRating = 3
		case result.Coverage >= 40:
			result.SuggestedRating = 2
		default:
			result.SuggestedRating = 1
		}
	}

	renv.Success(c, gin.H{
		"points":          result.Points,
		"coverage":        result.Coverage,
		"suggestedRating": result.SuggestedRating,
	})
}
