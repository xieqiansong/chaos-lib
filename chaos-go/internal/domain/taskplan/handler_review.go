package taskplan

import (
	"errors"
	"net/http"

	"chaos-go/internal/framework/envelope"
	"chaos-go/internal/framework/httpx"
	renv "chaos-go/internal/framework/resp"

	"github.com/gin-gonic/gin"
)

// GetTaskPlanRaw 读取计划关联的 raw_link 原文内容（服务端代理拉取，规避浏览器跨域）。
func GetTaskPlanRaw(c *gin.Context) {
	id, ok := envelope.ID(c)
	if !ok {
		return
	}
	raw, err := FetchPlanRaw(id)
	if err != nil {
		if errors.Is(err, ErrNoRawLink) {
			renv.Error(c, http.StatusNotFound, err.Error())
			return
		}
		// 上游拉取失败：网关错误，区别于本服务内部错误。
		renv.Error(c, http.StatusBadGateway, err.Error())
		return
	}
	renv.Success(c, raw)
}

// ReviewTaskPlan 主动回忆式复习：先在前端隐藏原文、用户填写回忆，再提交评分。
// 评分驱动 FSRS 调度（interval 类型），并把用户的回忆内容写入任务 remark 以便后续对比。
func ReviewTaskPlan(c *gin.Context) {
	id, ok := envelope.ID(c)
	if !ok {
		return
	}
	var req ReviewRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		renv.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	resp, err := ReviewPlan(id, req)
	if err != nil {
		httpx.MapError(c, err, errRules)
		return
	}
	renv.Success(c, resp)
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
		renv.Error(c, http.StatusBadRequest, "请求格式错误: "+err.Error())
		return
	}
	score, err := ScoreReview(req.Original, req.Answer)
	if err != nil {
		if errors.Is(err, ErrInvalidState) {
			renv.Error(c, http.StatusBadRequest, err.Error())
			return
		}
		// DeepSeek 调用失败或返回无法解析：网关错误。
		renv.Error(c, http.StatusBadGateway, err.Error())
		return
	}
	renv.Success(c, score)
}
