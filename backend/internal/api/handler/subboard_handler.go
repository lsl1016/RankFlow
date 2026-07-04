package handler

import (
	"github.com/gin-gonic/gin"

	"rankflow/internal/dto"
)

func (h *Handler) ListSubBoards(c *gin.Context) {
	rankID, err := pathRankID(c)
	if err != nil {
		fail(c, wrapValidation(err))
		return
	}
	rows, err := h.svc.ListSubBoards(c.Request.Context(), rankID)
	if err != nil {
		fail(c, err)
		return
	}
	ok(c, dto.FromSubBoards(rows))
}

func (h *Handler) ResolveSubBoard(c *gin.Context) {
	rankID, err := pathRankID(c)
	if err != nil {
		fail(c, wrapValidation(err))
		return
	}
	var req dto.ResolveSubBoardRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, wrapValidation(err))
		return
	}
	sb, err := h.svc.ResolveSubBoard(c.Request.Context(), rankID, req.Dimensions, req.Timestamp)
	if err != nil {
		fail(c, err)
		return
	}
	ok(c, dto.FromSubBoard(sb))
}

func (h *Handler) SetSubBoardStatus(c *gin.Context) {
	rankID, err := pathRankID(c)
	if err != nil {
		fail(c, wrapValidation(err))
		return
	}
	var req dto.SetSubBoardStatusRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, wrapValidation(err))
		return
	}
	if err := h.svc.SetSubBoardStatus(c.Request.Context(), rankID, req.TypeID, req.Status); err != nil {
		fail(c, err)
		return
	}
	ok(c, gin.H{"rankId": rankID, "typeId": req.TypeID, "status": req.Status})
}
