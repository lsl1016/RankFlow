package handler

import (
	"github.com/gin-gonic/gin"

	"rankflow/internal/service"
)

// AddScore POST /api/ranks/:rankId/score/add
func (h *Handler) AddScore(c *gin.Context) {
	h.metrics.IncWrite()
	rankID, err := pathRankID(c)
	if err != nil {
		fail(c, wrapValidation(err))
		return
	}
	var in service.AddScoreInput
	if err := c.ShouldBindJSON(&in); err != nil {
		fail(c, wrapValidation(err))
		return
	}
	res, err := h.svc.AddScore(c.Request.Context(), rankID, &in)
	if err != nil {
		fail(c, err)
		return
	}
	ok(c, res)
}

// SetScore POST /api/ranks/:rankId/score/set
func (h *Handler) SetScore(c *gin.Context) {
	h.metrics.IncWrite()
	rankID, err := pathRankID(c)
	if err != nil {
		fail(c, wrapValidation(err))
		return
	}
	var in service.AddScoreInput
	if err := c.ShouldBindJSON(&in); err != nil {
		fail(c, wrapValidation(err))
		return
	}
	res, err := h.svc.SetScore(c.Request.Context(), rankID, &in)
	if err != nil {
		fail(c, err)
		return
	}
	ok(c, res)
}

// BatchAddScore POST /api/ranks/:rankId/score/batch  body {items:[...]}
func (h *Handler) BatchAddScore(c *gin.Context) {
	rankID, err := pathRankID(c)
	if err != nil {
		fail(c, wrapValidation(err))
		return
	}
	var body struct {
		Items []service.AddScoreInput `json:"items"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		fail(c, wrapValidation(err))
		return
	}
	h.metrics.IncWrite()
	res, err := h.svc.BatchAddScore(c.Request.Context(), rankID, body.Items)
	if err != nil {
		fail(c, err)
		return
	}
	ok(c, gin.H{"results": res})
}
