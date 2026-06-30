package handler

import (
	"strconv"

	"github.com/gin-gonic/gin"

	"rankflow/internal/service"
)

// CreateRank POST /api/ranks
func (h *Handler) CreateRank(c *gin.Context) {
	var in service.CreateRankInput
	if err := c.ShouldBindJSON(&in); err != nil {
		fail(c, wrapValidation(err))
		return
	}
	id, err := h.svc.CreateRank(c.Request.Context(), &in)
	if err != nil {
		fail(c, err)
		return
	}
	ok(c, gin.H{"rankId": id})
}

// UpdateRank PUT /api/ranks/:rankId
func (h *Handler) UpdateRank(c *gin.Context) {
	rankID, err := pathRankID(c)
	if err != nil {
		fail(c, wrapValidation(err))
		return
	}
	var in service.CreateRankInput
	if err := c.ShouldBindJSON(&in); err != nil {
		fail(c, wrapValidation(err))
		return
	}
	if err := h.svc.UpdateRank(c.Request.Context(), rankID, &in); err != nil {
		fail(c, err)
		return
	}
	ok(c, gin.H{"rankId": rankID})
}

// ListRanks GET /api/ranks
func (h *Handler) ListRanks(c *gin.Context) {
	name := c.Query("name")
	bizCode := c.Query("bizCode")
	var statusPtr *int
	if s := c.Query("status"); s != "" {
		if v, err := strconv.Atoi(s); err == nil {
			statusPtr = &v
		}
	}
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	size, _ := strconv.Atoi(c.DefaultQuery("size", "20"))

	rows, total, err := h.svc.ListRanks(c.Request.Context(), name, bizCode, statusPtr, page, size)
	if err != nil {
		fail(c, err)
		return
	}
	ok(c, gin.H{"total": total, "list": rows, "page": page, "size": size})
}

// GetRank GET /api/ranks/:rankId
func (h *Handler) GetRank(c *gin.Context) {
	rankID, err := pathRankID(c)
	if err != nil {
		fail(c, wrapValidation(err))
		return
	}
	detail, err := h.svc.GetRankDetail(c.Request.Context(), rankID)
	if err != nil {
		fail(c, err)
		return
	}
	ok(c, detail)
}

// SetStatus POST /api/ranks/:rankId/status  body {status:int}
func (h *Handler) SetStatus(c *gin.Context) {
	rankID, err := pathRankID(c)
	if err != nil {
		fail(c, wrapValidation(err))
		return
	}
	var body struct {
		Status int `json:"status"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		fail(c, wrapValidation(err))
		return
	}
	if err := h.svc.SetStatus(c.Request.Context(), rankID, body.Status); err != nil {
		fail(c, err)
		return
	}
	ok(c, gin.H{"rankId": rankID, "status": body.Status})
}
