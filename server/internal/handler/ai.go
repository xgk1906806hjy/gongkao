package handler

import (
	"errors"
	"strconv"

	"gongkao/internal/middleware"
	"gongkao/internal/pkg"
	"gongkao/internal/service"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func (h *Handlers) AIStatus(c *gin.Context) {
	pkg.OK(c, h.AI.Status(middleware.UserID(c)))
}

func (h *Handlers) GetAIProfile(c *gin.Context) {
	p, err := h.AI.GetProfile(middleware.UserID(c))
	if err != nil {
		pkg.Internal(c, err.Error())
		return
	}
	pkg.OK(c, p)
}

func (h *Handlers) SaveAIProfile(c *gin.Context) {
	var req service.ProfileInput
	if err := c.ShouldBindJSON(&req); err != nil {
		pkg.BadRequest(c, "invalid body")
		return
	}
	p, err := h.AI.SaveProfile(middleware.UserID(c), req)
	if err != nil {
		pkg.BadRequest(c, err.Error())
		return
	}
	pkg.OK(c, p)
}

func (h *Handlers) DeleteAIProfile(c *gin.Context) {
	if err := h.AI.DeleteProfile(middleware.UserID(c)); err != nil {
		pkg.Internal(c, err.Error())
		return
	}
	pkg.OK(c, gin.H{"ok": true})
}

func (h *Handlers) AnalyzeResume(c *gin.Context) {
	var req service.ResumeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		pkg.BadRequest(c, "invalid body")
		return
	}
	res, err := h.AI.AnalyzeResume(c.Request.Context(), middleware.UserID(c), req)
	if err != nil {
		pkg.BadRequest(c, err.Error())
		return
	}
	pkg.OK(c, res)
}

func (h *Handlers) GenerateStudyPlan(c *gin.Context) {
	var req service.PlanRequest
	_ = c.ShouldBindJSON(&req)
	res, err := h.AI.StudyPlan(c.Request.Context(), middleware.UserID(c), req)
	if err != nil {
		pkg.BadRequest(c, err.Error())
		return
	}
	pkg.OK(c, res)
}

func (h *Handlers) AIAsk(c *gin.Context) {
	var req struct {
		Question string `json:"question"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		pkg.BadRequest(c, "invalid body")
		return
	}
	res, err := h.AI.Ask(c.Request.Context(), middleware.UserID(c), req.Question)
	if err != nil {
		pkg.BadRequest(c, err.Error())
		return
	}
	pkg.OK(c, res)
}

func (h *Handlers) ListAIReports(c *gin.Context) {
	page, size := pkg.PageParams(c)
	res, err := h.AI.ListReports(middleware.UserID(c), c.Query("kind"), page, size)
	if err != nil {
		pkg.Internal(c, err.Error())
		return
	}
	pkg.OK(c, res)
}

func (h *Handlers) GetAIReport(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	res, err := h.AI.GetReport(middleware.UserID(c), id)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		pkg.NotFound(c, "report not found")
		return
	}
	if err != nil {
		pkg.Internal(c, err.Error())
		return
	}
	pkg.OK(c, res)
}
