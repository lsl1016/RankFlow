package handler

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"rankflow/internal/observability"
	"rankflow/internal/service"
)

// wrapValidation tags a binding/parse error as a validation error so fail()
// maps it to HTTP 400.
func wrapValidation(err error) error {
	return fmt.Errorf("%w: %v", service.ErrValidation, err)
}

type Handler struct {
	svc     *service.Service
	metrics *observability.Metrics
}

func New(svc *service.Service, metrics *observability.Metrics) *Handler {
	return &Handler{svc: svc, metrics: metrics}
}

// resp is the unified response envelope {code, message, data}.
func ok(c *gin.Context, data any) {
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "success", "data": data})
}

func fail(c *gin.Context, err error) {
	status := http.StatusInternalServerError
	code := 5000
	switch {
	case errors.Is(err, service.ErrValidation):
		status, code = http.StatusBadRequest, 4000
	case errors.Is(err, service.ErrNotFound):
		status, code = http.StatusNotFound, 4040
	case errors.Is(err, service.ErrNotOnline):
		status, code = http.StatusConflict, 4090
	}
	c.JSON(status, gin.H{"code": code, "message": err.Error(), "data": nil})
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
