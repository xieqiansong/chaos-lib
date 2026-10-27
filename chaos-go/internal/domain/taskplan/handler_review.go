package taskplan

import (
	"errors"
	"net/http"

	"chaos-go/internal/framework/envelope"
	"chaos-go/internal/framework/httpx"
	renv "chaos-go/internal/framework/resp"

	"chaos-go/internal/framework/web"
)

func GetTaskPlanRaw(c *web.Context) {
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

		renv.Error(c, http.StatusBadGateway, err.Error())
		return
	}
	renv.Success(c, raw)
}

func ReviewTaskPlan(c *web.Context) {
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

func AiReviewScore(c *web.Context) {
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

		renv.Error(c, http.StatusBadGateway, err.Error())
		return
	}
	renv.Success(c, score)
}
