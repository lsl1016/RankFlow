package handler

import (
	"strconv"

	"github.com/gin-gonic/gin"
)

func queryTS(c *gin.Context) int64 {
	ts, _ := strconv.ParseInt(c.Query("timestamp"), 10, 64)
	return ts
}

// Top GET /api/ranks/:rankId/top
func (h *Handler) Top(c *gin.Context) {
	h.metrics.IncRead()
	rankID, err := pathRankID(c)
	if err != nil {
		fail(c, wrapValidation(err))
		return
	}
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "100"))
	res, err := h.svc.QueryTop(c.Request.Context(), rankID, parseDimensions(c), queryTS(c), offset, limit)
	if err != nil {
		fail(c, err)
		return
	}
	ok(c, res)
}

// MemberRank GET /api/ranks/:rankId/members/:itemId/rank
func (h *Handler) MemberRank(c *gin.Context) {
	h.metrics.IncRead()
	rankID, err := pathRankID(c)
	if err != nil {
		fail(c, wrapValidation(err))
		return
	}
	itemID := c.Param("itemId")
	res, err := h.svc.QueryMemberRank(c.Request.Context(), rankID, itemID, parseDimensions(c), queryTS(c))
	if err != nil {
		fail(c, err)
		return
	}
	ok(c, res)
}

// Around GET /api/ranks/:rankId/members/:itemId/around
func (h *Handler) Around(c *gin.Context) {
	h.metrics.IncRead()
	rankID, err := pathRankID(c)
	if err != nil {
		fail(c, wrapValidation(err))
		return
	}
	itemID := c.Param("itemId")
	before, _ := strconv.Atoi(c.DefaultQuery("before", "5"))
	after, _ := strconv.Atoi(c.DefaultQuery("after", "5"))
	res, err := h.svc.QueryAround(c.Request.Context(), rankID, itemID, parseDimensions(c), queryTS(c), before, after)
	if err != nil {
		fail(c, err)
		return
	}
	ok(c, res)
}

// Stats GET /api/ranks/:rankId/stats
func (h *Handler) Stats(c *gin.Context) {
	rankID, err := pathRankID(c)
	if err != nil {
		fail(c, wrapValidation(err))
		return
	}
	res, err := h.svc.Stats(c.Request.Context(), rankID, parseDimensions(c), queryTS(c))
	if err != nil {
		fail(c, err)
		return
	}
	snap := h.metrics.Snapshot()
	ok(c, gin.H{
		"rankId":       res.RankID,
		"typeId":       res.TypeID,
		"memberCount":  res.MemberCount,
		"writeQps":     snap.WriteQPS,
		"readQps":      snap.ReadQPS,
		"cacheHitRate": snap.CacheHitRate,
		"writeCount":   snap.WriteCount,
		"readCount":    snap.ReadCount,
	})
}
