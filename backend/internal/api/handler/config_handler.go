package handler

import (
	"strconv"

	"github.com/gin-gonic/gin"

	"rankflow/internal/dto"
)

// CreateRank 创建榜单
//
//	@Summary		创建榜单
//	@Description	按配置创建一个新榜单，返回自动分配的榜单 ID
//	@Tags			榜单配置
//	@Accept			json
//	@Produce		json
//	@Param			body	body		dto.CreateRankRequest	true	"榜单配置"
//	@Success		200		{object}	dto.Response{data=dto.CreateRankData}
//	@Failure		400		{object}	dto.Response
//	@Router			/ranks [post]
func (h *Handler) CreateRank(c *gin.Context) {
	var req dto.CreateRankRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, wrapValidation(err))
		return
	}
	id, err := h.svc.CreateRank(c.Request.Context(), req.ToServiceInput())
	if err != nil {
		fail(c, err)
		return
	}
	ok(c, dto.CreateRankData{RankID: id})
}

// UpdateRank 编辑榜单
//
//	@Summary		编辑榜单
//	@Description	全量更新指定榜单的配置
//	@Tags			榜单配置
//	@Accept			json
//	@Produce		json
//	@Param			rankId	path		int						true	"榜单 ID"
//	@Param			body	body		dto.CreateRankRequest	true	"榜单配置"
//	@Success		200		{object}	dto.Response{data=dto.CreateRankData}
//	@Failure		400		{object}	dto.Response
//	@Failure		404		{object}	dto.Response
//	@Router			/ranks/{rankId} [put]
func (h *Handler) UpdateRank(c *gin.Context) {
	rankID, err := pathRankID(c)
	if err != nil {
		fail(c, wrapValidation(err))
		return
	}
	var req dto.CreateRankRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, wrapValidation(err))
		return
	}
	if err := h.svc.UpdateRank(c.Request.Context(), rankID, req.ToServiceInput()); err != nil {
		fail(c, err)
		return
	}
	ok(c, dto.CreateRankData{RankID: rankID})
}

// ListRanks 榜单列表
//
//	@Summary		榜单列表
//	@Description	按名称 / 业务线 / 状态分页查询榜单
//	@Tags			榜单配置
//	@Produce		json
//	@Param			name		query		string	false	"榜单名称模糊匹配"
//	@Param			bizCode		query		string	false	"业务线编码"
//	@Param			status		query		int		false	"状态：0草稿 1上线 2下线 3归档"
//	@Param			page		query		int		false	"页码，缺省 1"
//	@Param			size		query		int		false	"每页大小，缺省 20"
//	@Success		200			{object}	dto.Response{data=dto.RankListData}
//	@Router			/ranks [get]
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
	ok(c, dto.FromRankConfigList(rows, total, page, size))
}

// GetRank 榜单详情
//
//	@Summary		榜单详情
//	@Description	返回榜单基础配置、横向维度与时间维度配置
//	@Tags			榜单配置
//	@Produce		json
//	@Param			rankId	path		int	true	"榜单 ID"
//	@Success		200		{object}	dto.Response{data=dto.RankDetailData}
//	@Failure		404		{object}	dto.Response
//	@Router			/ranks/{rankId} [get]
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
	ok(c, dto.FromResolvedConfig(detail))
}

// SetStatus 上下线 / 归档
//
//	@Summary		榜单上下线 / 归档
//	@Description	变更榜单状态：0草稿 1上线 2下线 3归档
//	@Tags			榜单配置
//	@Accept			json
//	@Produce		json
//	@Param			rankId	path		int						true	"榜单 ID"
//	@Param			body	body		dto.SetStatusRequest	true	"目标状态"
//	@Success		200		{object}	dto.Response
//	@Failure		400		{object}	dto.Response
//	@Failure		404		{object}	dto.Response
//	@Router			/ranks/{rankId}/status [post]
func (h *Handler) SetStatus(c *gin.Context) {
	rankID, err := pathRankID(c)
	if err != nil {
		fail(c, wrapValidation(err))
		return
	}
	var req dto.SetStatusRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, wrapValidation(err))
		return
	}
	if err := h.svc.SetStatus(c.Request.Context(), rankID, req.Status); err != nil {
		fail(c, err)
		return
	}
	ok(c, gin.H{"rankId": rankID, "status": req.Status})
}
