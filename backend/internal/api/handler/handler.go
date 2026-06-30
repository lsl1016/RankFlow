package handler

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"rankflow/internal/dto"
	"rankflow/internal/observability"
	"rankflow/internal/service"
)

type Handler struct {
	svc     *service.Service
	metrics *observability.Metrics
}

func New(svc *service.Service, metrics *observability.Metrics) *Handler {
	return &Handler{svc: svc, metrics: metrics}
}

// wrapValidation tags a binding/parse error as a validation error so fail()
// maps it to HTTP 400.
func wrapValidation(err error) error {
	return fmt.Errorf("%w: %v", service.ErrValidation, err)
}

// ok writes a unified success response.
func ok(c *gin.Context, data any) {
	c.JSON(http.StatusOK, dto.Success(data))
}

// fail maps a domain error to the appropriate HTTP status + business code and
// writes a unified failure response.
func fail(c *gin.Context, err error) {
	status, code := http.StatusInternalServerError, dto.CodeInternal
	switch {
	case errors.Is(err, service.ErrValidation):
		status, code = http.StatusBadRequest, dto.CodeValidation
	case errors.Is(err, service.ErrNotFound):
		status, code = http.StatusNotFound, dto.CodeNotFound
	case errors.Is(err, service.ErrNotOnline):
		status, code = http.StatusConflict, dto.CodeConflict
	}
	c.JSON(status, dto.Fail(code, err.Error()))
}

func pathRankID(c *gin.Context) (int64, error) {
	return strconv.ParseInt(c.Param("rankId"), 10, 64)
}

// parseDimensions extracts dimension key/values from query params prefixed with
// "dim_", e.g. ?dim_business_id=community&dim_category_id=tech.
func parseDimensions(c *gin.Context) map[string]string {
	out := map[string]string{}
	for k, v := range c.Request.URL.Query() {
		if len(k) > 4 && k[:4] == "dim_" && len(v) > 0 {
			out[k[4:]] = v[0]
		}
	}
	return out
}
