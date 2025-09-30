package http

import (
	"strconv"

	"simple-securities/api/error_code"
	"simple-securities/api/http/handle"
	"simple-securities/api/http/validator"
	"simple-securities/application/dto"
	"simple-securities/application/service"
	"simple-securities/pkg/logger"

	"github.com/gin-gonic/gin"
)

type ExampleHandler interface {
	Create(ctx *gin.Context)
	Delete(ctx *gin.Context)
	Update(ctx *gin.Context)
	Get(ctx *gin.Context)
	FindByName(ctx *gin.Context)
}

type exampleHandler struct {
	router         *gin.Engine
	exampleService service.IExampleService
}

func NewExampleHandler(router *gin.Engine, exampleService service.IExampleService) {
	h := &exampleHandler{
		router:         router,
		exampleService: exampleService,
	}
	h.initRoutes()
}

func (h *exampleHandler) initRoutes() {
	v1 := h.router.Group("/api/v1/examples")
	{
		v1.POST("", h.Create)
		v1.GET("/:id", h.Get)
		v1.PUT("/:id", h.Update)
		v1.DELETE("/:id", h.Delete)
		v1.GET("/name/:name", h.FindByName)
	}
}

// Create handles the creation of a new example.
func (h *exampleHandler) Create(ctx *gin.Context) {
	response := handle.NewResponse(ctx)
	body := dto.CreateExampleReq{}

	if valid, errs := validator.BindAndValid(ctx, &body, ctx.ShouldBindJSON); !valid {
		logger.SugaredLogger.Errorf("Create.BindAndValid errs: %v", errs)
		err := error_code.InvalidParams.WithDetails(errs.Errors()...)
		response.ToErrorResponse(err)
		return
	}

	createdExample, err := h.exampleService.Create(ctx, body.Name, body.Alias)
	if err != nil {
		logger.SugaredLogger.Errorf("Create.exampleService.Create err: %v", err)
		response.ToErrorResponse(error_code.ServerError)
		return
	}

	response.ToResponse(createdExample)
}

// Get handles retrieving an example by its ID.
func (h *exampleHandler) Get(ctx *gin.Context) {
	response := handle.NewResponse(ctx)
	id, err := strconv.Atoi(ctx.Param("id"))
	if err != nil {
		logger.SugaredLogger.Errorf("Get.Invalid ID: %v", err)
		response.ToErrorResponse(error_code.InvalidParams.WithDetails("invalid id"))
		return
	}

	example, err := h.exampleService.Get(ctx, id)
	if err != nil {
		logger.SugaredLogger.Errorf("Get.exampleService.Get err: %v", err)
		response.ToErrorResponse(error_code.ServerError)
		return
	}

	response.ToResponse(example)
}

// FindByName handles retrieving an example by its name.
func (h *exampleHandler) FindByName(ctx *gin.Context) {
	response := handle.NewResponse(ctx)
	name := ctx.Param("name")

	example, err := h.exampleService.FindByName(ctx, name)
	if err != nil {
		logger.SugaredLogger.Errorf("FindByName.exampleService.FindByName err: %v", err)
		response.ToErrorResponse(error_code.ServerError)
		return
	}

	response.ToResponse(example)
}

// Update handles updating an existing example.
func (h *exampleHandler) Update(ctx *gin.Context) {
	response := handle.NewResponse(ctx)
	id, err := strconv.Atoi(ctx.Param("id"))
	if err != nil {
		logger.SugaredLogger.Errorf("Update.Invalid ID: %v", err)
		response.ToErrorResponse(error_code.InvalidParams.WithDetails("invalid id"))
		return
	}

	body := dto.UpdateExampleReq{}
	if valid, errs := validator.BindAndValid(ctx, &body, ctx.ShouldBindJSON); !valid {
		logger.SugaredLogger.Errorf("Update.BindAndValid errs: %v", errs)
		err := error_code.InvalidParams.WithDetails(errs.Errors()...)
		response.ToErrorResponse(err)
		return
	}

	if err := h.exampleService.Update(ctx, id, body.Name, body.Alias); err != nil {
		logger.SugaredLogger.Errorf("Update.exampleService.Update err: %v", err)
		response.ToErrorResponse(error_code.ServerError)
		return
	}

	response.ToSuccess()
}

// Delete handles deleting an example by its ID.
func (h *exampleHandler) Delete(ctx *gin.Context) {
	response := handle.NewResponse(ctx)
	id, err := strconv.Atoi(ctx.Param("id"))
	if err != nil {
		logger.SugaredLogger.Errorf("Delete.Invalid ID: %v", err)
		response.ToErrorResponse(error_code.InvalidParams.WithDetails("invalid id"))
		return
	}

	if err := h.exampleService.Delete(ctx, id); err != nil {
		logger.SugaredLogger.Errorf("Delete.exampleService.Delete err: %v", err)
		response.ToErrorResponse(error_code.ServerError)
		return
	}

	response.ToSuccess()
}
