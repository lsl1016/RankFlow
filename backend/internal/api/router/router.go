package router

import (
	"github.com/gin-gonic/gin"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
	"go.uber.org/zap"

	"rankflow/internal/api/handler"
	"rankflow/internal/api/middleware"
)

// New builds the Gin engine with public query routes plus isolated writer and
// administrator permission groups. URLs stay backward compatible.
func New(h *handler.Handler, log *zap.Logger, auth middleware.AuthConfig) *gin.Engine {
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.Use(middleware.RequestContext(log), middleware.AccessLog(log), middleware.Recovery(log), middleware.CORS())

	r.GET("/healthz", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok"})
	})

	// Swagger describes configuration/write APIs, so production access is admin-only.
	r.GET("/swagger/*any", middleware.RequireAdmin(auth), ginSwagger.WrapHandler(swaggerFiles.Handler))

	api := r.Group("/api")

	// Ranking reads are intentionally public and have no subboard-persistence side effects.
	api.GET("/ranks/:rankId/top", h.Top)
	api.GET("/ranks/:rankId/members/:itemId/rank", h.MemberRank)
	api.GET("/ranks/:rankId/members/:itemId/around", h.Around)
	api.GET("/ranks/:rankId/stats", h.Stats)

	writer := api.Group("")
	writer.Use(middleware.RequireWriter(auth))
	{
		writer.POST("/ranks/:rankId/score/add", h.AddScore)
		writer.POST("/ranks/:rankId/score/set", h.SetScore)
		writer.POST("/ranks/:rankId/score/batch", h.BatchAddScore)
	}

	admin := api.Group("")
	admin.Use(middleware.RequireAdmin(auth))
	{
		admin.POST("/ranks", h.CreateRank)
		admin.GET("/ranks", h.ListRanks)
		admin.GET("/ranks/:rankId", h.GetRank)
		admin.PUT("/ranks/:rankId", h.UpdateRank)
		admin.POST("/ranks/:rankId/status", h.SetStatus)
		admin.GET("/ranks/:rankId/subboards", h.ListSubBoards)
		admin.POST("/ranks/:rankId/subboards", h.ResolveSubBoard)
		admin.POST("/ranks/:rankId/subboards/status", h.SetSubBoardStatus)
	}

	return r
}
