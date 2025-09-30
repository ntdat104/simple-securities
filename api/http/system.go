package http

import (
	"simple-securities/api/http/handle"
	"simple-securities/application/service"

	"github.com/gin-gonic/gin"
)

type SystemHandler interface {
	GetTime(ctx *gin.Context)
}

type systemHandler struct {
	router        *gin.Engine
	systemService service.SystemService
}

func NewSystemHandler(router *gin.Engine, systemService service.SystemService) {
	h := &systemHandler{
		router:        router,
		systemService: systemService,
	}
	h.initRoutes()
}

func (h *systemHandler) initRoutes() {
	v1 := h.router.Group("/api/v1/system")
	{
		v1.GET("/time", h.GetTime)
	}
}

func (h *systemHandler) GetTime(ctx *gin.Context) {
	response := handle.NewResponse(ctx)
	response.ToResponse(h.systemService.GetTime())
}
