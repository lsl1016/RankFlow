package router

import (
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"rankflow/internal/api/handler"
	"rankflow/internal/api/middleware"
)

// New builds the Gin engine with all routes wired.
func New(h *handler.Handler, log *zap.Logger) *gin.Engine {
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.Use(gin.Recovery(), middleware.CORS(), middleware.AccessLog(log))

	r.GET("/healthz", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok"})
	})

	api := r.Group("/api")
	{
		api.POST("/ranks", h.CreateRank)
		api.GET("/ranks", h.ListRanks)
		api.GET("/ranks/:rankId", h.GetRank)
		api.PUT("/ranks/:rankId", h.UpdateRank)
		api.POST("/ranks/:rankId/status", h.SetStatus)

		api.POST("/ranks/:rankId/score/add", h.AddScore)
		api.POST("/ranks/:rankId/score/set", h.SetScore)
		api.POST("/ranks/:rankId/score/batch", h.BatchAddScore)

		api.GET("/ranks/:rankId/top", h.Top)
		api.GET("/ranks/:rankId/members/:itemId/rank", h.MemberRank)
		api.GET("/ranks/:rankId/members/:itemId/around", h.Around)
		api.GET("/ranks/:rankId/stats", h.Stats)
	}

	return r
}
