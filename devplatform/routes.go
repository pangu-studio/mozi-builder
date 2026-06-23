package devplatform

import "github.com/gin-gonic/gin"

// RegisterRoutes mounts mozi builder HTTP routes into a host Gin router.
// The host application owns the HTTP server lifecycle, auth middleware,
// logging, CORS, and deployment concerns.
func RegisterRoutes(api gin.IRoutes, h *Handler) {
	api.GET("/models", h.ListModels)
	api.POST("/models", h.CreateModel)
	api.GET("/apis", h.ListAPIAssets)
	api.POST("/apis/overrides", h.SaveAPIEndpointOverride)
	api.GET("/dictionaries/:dictionary/items", h.ListDesignDictionaryItems)
	api.POST("/dictionaries/:dictionary/items", h.SaveDesignDictionaryItem)
	api.PUT("/dictionaries/:dictionary/items/:value", h.SaveDesignDictionaryItem)
	api.DELETE("/dictionaries/:dictionary/items/:value", h.DeleteDesignDictionaryItem)
	api.POST("/modules", h.CreateModule)
	api.PUT("/modules/:module", h.UpdateModule)
	api.DELETE("/modules/:module", h.DeleteModule)

	api.GET("/models/er", h.ERDiagram)

	api.GET("/modules/:module/models/:name", h.GetModel)
	api.GET("/modules/:module/models/:name/history", h.GetModelHistory)
	api.PUT("/modules/:module/models/:name", h.UpdateModel)
	api.DELETE("/modules/:module/models/:name", h.DeleteModel)
	api.POST("/modules/:module/models/:name/validate", h.Validate)
	api.GET("/modules/:module/models/:name/diff", h.GetDiff)
	api.GET("/modules/:module/models/:name/change-plan", h.GetChangePlan)
	api.POST("/modules/:module/models/:name/sync", h.SyncModel)
}

// RegisterUnavailableRoutes mounts the same route surface to one fallback
// handler. Host applications can use it when mozi is configured off or the
// design database is unavailable.
func RegisterUnavailableRoutes(api gin.IRoutes, fallback gin.HandlerFunc) {
	api.GET("/models", fallback)
	api.POST("/models", fallback)
	api.GET("/apis", fallback)
	api.POST("/apis/overrides", fallback)
	api.GET("/dictionaries/:dictionary/items", fallback)
	api.POST("/dictionaries/:dictionary/items", fallback)
	api.PUT("/dictionaries/:dictionary/items/:value", fallback)
	api.DELETE("/dictionaries/:dictionary/items/:value", fallback)
	api.POST("/modules", fallback)
	api.PUT("/modules/:module", fallback)
	api.DELETE("/modules/:module", fallback)

	api.GET("/models/er", fallback)

	api.GET("/modules/:module/models/:name", fallback)
	api.GET("/modules/:module/models/:name/history", fallback)
	api.PUT("/modules/:module/models/:name", fallback)
	api.DELETE("/modules/:module/models/:name", fallback)
	api.POST("/modules/:module/models/:name/validate", fallback)
	api.GET("/modules/:module/models/:name/diff", fallback)
	api.GET("/modules/:module/models/:name/change-plan", fallback)
	api.POST("/modules/:module/models/:name/sync", fallback)
}
