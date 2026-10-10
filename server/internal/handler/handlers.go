package handler

import (
	"errors"
	"strconv"

	"gongkao/internal/middleware"
	"gongkao/internal/model"
	"gongkao/internal/pkg"
	"gongkao/internal/service"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type Handlers struct {
	Auth *service.AuthService
	Ann  *service.AnnouncementService
	Exam *service.ExamTodoService
	Prac *service.PracticeService
	Aff  *service.AffairsService
	JWT  *pkg.JWTManager
	DB   *gorm.DB
}

func (h *Handlers) Health(c *gin.Context) {
	pkg.OK(c, gin.H{"status": "up"})
}

func (h *Handlers) WechatLogin(c *gin.Context) {
	var req struct {
		Code      string `json:"code"`
		Nickname  string `json:"nickname"`
		AvatarURL string `json:"avatar_url"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		pkg.BadRequest(c, "invalid body")
		return
	}
	if req.Code == "" {
		req.Code = "dev"
	}
	res, err := h.Auth.WechatLogin(c.Request.Context(), req.Code, req.Nickname, req.AvatarURL)
	if err != nil {
		pkg.Fail(c, 500, 50010, err.Error())
		return
	}
	pkg.OK(c, res)
}

func (h *Handlers) Refresh(c *gin.Context) {
	var req struct {
		RefreshToken string `json:"refresh_token"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.RefreshToken == "" {
		pkg.BadRequest(c, "refresh_token required")
		return
	}
	access, refresh, err := h.Auth.Refresh(req.RefreshToken)
	if err != nil {
		pkg.Unauthorized(c, err.Error())
		return
	}
	pkg.OK(c, gin.H{"access_token": access, "refresh_token": refresh})
}

func (h *Handlers) Me(c *gin.Context) {
	u, err := h.Auth.GetUser(middleware.UserID(c))
	if err != nil {
		pkg.NotFound(c, "user not found")
		return
	}
	pkg.OK(c, u)
}

func (h *Handlers) UpdateMe(c *gin.Context) {
	var req struct {
		Nickname  string `json:"nickname"`
		AvatarURL string `json:"avatar_url"`
	}
	_ = c.ShouldBindJSON(&req)
	u, err := h.Auth.UpdateProfile(middleware.UserID(c), req.Nickname, req.AvatarURL)
	if err != nil {
		pkg.Internal(c, err.Error())
		return
	}
	pkg.OK(c, u)
}

func (h *Handlers) ListAnnouncements(c *gin.Context) {
	page, size := pkg.PageParams(c)
	res, err := h.Ann.List(service.AnnouncementQuery{
		Type: c.Query("type"), Region: c.Query("region"), Keyword: c.Query("keyword"),
		Page: page, PageSize: size,
	})
	if err != nil {
		pkg.Internal(c, err.Error())
		return
	}
	pkg.OK(c, res)
}

func (h *Handlers) GetAnnouncement(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	a, err := h.Ann.Get(id, false)
	if err != nil {
		pkg.NotFound(c, "not found")
		return
	}
	pkg.OK(c, a)
}

func (h *Handlers) ListMyExams(c *gin.Context) {
	list, err := h.Exam.ListExams(middleware.UserID(c))
	if err != nil {
		pkg.Internal(c, err.Error())
		return
	}
	pkg.OK(c, gin.H{"list": list})
}

func (h *Handlers) JoinExam(c *gin.Context) {
	var req struct {
		AnnouncementID uint64 `json:"announcement_id"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.AnnouncementID == 0 {
		pkg.BadRequest(c, "announcement_id required")
		return
	}
	ue, err := h.Exam.Join(c.Request.Context(), middleware.UserID(c), req.AnnouncementID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			pkg.NotFound(c, "announcement not found")
			return
		}
		pkg.Internal(c, err.Error())
		return
	}
	pkg.OK(c, ue)
}

func (h *Handlers) LeaveExam(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("announcementId"), 10, 64)
	if err := h.Exam.Leave(c.Request.Context(), middleware.UserID(c), id); err != nil {
		pkg.NotFound(c, "not found")
		return
	}
	pkg.OK(c, gin.H{"ok": true})
}

func (h *Handlers) ListTodos(c *gin.Context) {
	list, err := h.Exam.ListTodos(middleware.UserID(c))
	if err != nil {
		pkg.Internal(c, err.Error())
		return
	}
	// group by announcement
	type group struct {
		AnnouncementID uint64             `json:"announcement_id"`
		Announcement   *model.Announcement `json:"announcement"`
		Todos          []model.Todo       `json:"todos"`
	}
	m := map[uint64]*group{}
	order := []uint64{}
	for _, t := range list {
		g, ok := m[t.AnnouncementID]
		if !ok {
			g = &group{AnnouncementID: t.AnnouncementID, Announcement: t.Announcement}
			m[t.AnnouncementID] = g
			order = append(order, t.AnnouncementID)
		}
		tt := t
		tt.Announcement = nil
		g.Todos = append(g.Todos, tt)
	}
	out := make([]group, 0, len(order))
	for _, id := range order {
		out = append(out, *m[id])
	}
	pkg.OK(c, gin.H{"list": out})
}

func (h *Handlers) PatchTodo(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	var req struct {
		Status string `json:"status"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.Status == "" {
		pkg.BadRequest(c, "status required")
		return
	}
	todo, err := h.Exam.PatchTodo(c.Request.Context(), middleware.UserID(c), id, req.Status)
	if err != nil {
		pkg.BadRequest(c, err.Error())
		return
	}
	pkg.OK(c, todo)
}

func (h *Handlers) ListCategories(c *gin.Context) {
	list, err := h.Prac.ListCategories()
	if err != nil {
		pkg.Internal(c, err.Error())
		return
	}
	pkg.OK(c, gin.H{"list": list})
}

func (h *Handlers) StartPractice(c *gin.Context) {
	var req service.StartSessionReq
	_ = c.ShouldBindJSON(&req)
	res, err := h.Prac.StartSession(middleware.UserID(c), req)
	if err != nil {
		pkg.BadRequest(c, err.Error())
		return
	}
	pkg.OK(c, res)
}

func (h *Handlers) SubmitPractice(c *gin.Context) {
	var req service.SubmitReq
	if err := c.ShouldBindJSON(&req); err != nil {
		pkg.BadRequest(c, "invalid body")
		return
	}
	res, err := h.Prac.Submit(middleware.UserID(c), req)
	if err != nil {
		pkg.BadRequest(c, err.Error())
		return
	}
	pkg.OK(c, res)
}

func (h *Handlers) ListWrong(c *gin.Context) {
	page, size := pkg.PageParams(c)
	res, err := h.Prac.ListWrong(middleware.UserID(c), page, size)
	if err != nil {
		pkg.Internal(c, err.Error())
		return
	}
	pkg.OK(c, res)
}

func (h *Handlers) DeleteWrong(c *gin.Context) {
	qid, _ := strconv.ParseUint(c.Param("questionId"), 10, 64)
	_ = h.Prac.RemoveWrong(middleware.UserID(c), qid)
	pkg.OK(c, gin.H{"ok": true})
}

func (h *Handlers) PracticeStats(c *gin.Context) {
	res, err := h.Prac.Stats(middleware.UserID(c))
	if err != nil {
		pkg.Internal(c, err.Error())
		return
	}
	pkg.OK(c, res)
}

func (h *Handlers) ListAffairs(c *gin.Context) {
	page, size := pkg.PageParams(c)
	res, err := h.Aff.List(service.AffairsQuery{
		Tag: c.Query("tag"), Keyword: c.Query("keyword"), Page: page, PageSize: size,
	})
	if err != nil {
		pkg.Internal(c, err.Error())
		return
	}
	pkg.OK(c, res)
}

func (h *Handlers) GetAffair(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	a, err := h.Aff.Get(id, middleware.UserID(c), false)
	if err != nil {
		pkg.NotFound(c, "not found")
		return
	}
	pkg.OK(c, a)
}

func (h *Handlers) FavoriteAffair(c *gin.Context) {
	var req struct {
		ArticleID uint64 `json:"article_id"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.ArticleID == 0 {
		pkg.BadRequest(c, "article_id required")
		return
	}
	if err := h.Aff.Favorite(middleware.UserID(c), req.ArticleID); err != nil {
		pkg.BadRequest(c, err.Error())
		return
	}
	pkg.OK(c, gin.H{"ok": true})
}

func (h *Handlers) UnfavoriteAffair(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	_ = h.Aff.Unfavorite(middleware.UserID(c), id)
	pkg.OK(c, gin.H{"ok": true})
}

func (h *Handlers) ListFavorites(c *gin.Context) {
	page, size := pkg.PageParams(c)
	res, err := h.Aff.ListFavorites(middleware.UserID(c), page, size)
	if err != nil {
		pkg.Internal(c, err.Error())
		return
	}
	pkg.OK(c, res)
}

// Admin

func (h *Handlers) AdminLogin(c *gin.Context) {
	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		pkg.BadRequest(c, "invalid body")
		return
	}
	token, err := h.Auth.AdminLogin(req.Username, req.Password)
	if err != nil {
		pkg.Unauthorized(c, err.Error())
		return
	}
	pkg.OK(c, gin.H{"access_token": token})
}

func (h *Handlers) AdminCreateAnnouncement(c *gin.Context) {
	var a model.Announcement
	if err := c.ShouldBindJSON(&a); err != nil {
		pkg.BadRequest(c, "invalid body")
		return
	}
	if err := h.Ann.Create(&a); err != nil {
		pkg.Internal(c, err.Error())
		return
	}
	pkg.OK(c, a)
}

func (h *Handlers) AdminUpdateAnnouncement(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	var updates map[string]interface{}
	if err := c.ShouldBindJSON(&updates); err != nil {
		pkg.BadRequest(c, "invalid body")
		return
	}
	a, err := h.Ann.Update(id, updates)
	if err != nil {
		pkg.Internal(c, err.Error())
		return
	}
	pkg.OK(c, a)
}

func (h *Handlers) AdminListAnnouncements(c *gin.Context) {
	page, size := pkg.PageParams(c)
	res, err := h.Ann.List(service.AnnouncementQuery{
		Type: c.Query("type"), Region: c.Query("region"), Keyword: c.Query("keyword"),
		Page: page, PageSize: size, AdminView: true,
	})
	if err != nil {
		pkg.Internal(c, err.Error())
		return
	}
	pkg.OK(c, res)
}

func (h *Handlers) AdminImportQuestions(c *gin.Context) {
	var req struct {
		Items []service.ImportQuestion `json:"items"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		pkg.BadRequest(c, "invalid body")
		return
	}
	n, err := h.Prac.ImportQuestions(req.Items)
	if err != nil {
		pkg.Internal(c, err.Error())
		return
	}
	pkg.OK(c, gin.H{"imported": n})
}

func (h *Handlers) AdminCreateAffair(c *gin.Context) {
	var a model.AffairsArticle
	if err := c.ShouldBindJSON(&a); err != nil {
		pkg.BadRequest(c, "invalid body")
		return
	}
	if err := h.Aff.Create(&a); err != nil {
		pkg.Internal(c, err.Error())
		return
	}
	pkg.OK(c, a)
}

func (h *Handlers) AdminUpdateAffair(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	var updates map[string]interface{}
	if err := c.ShouldBindJSON(&updates); err != nil {
		pkg.BadRequest(c, "invalid body")
		return
	}
	a, err := h.Aff.Update(id, updates)
	if err != nil {
		pkg.Internal(c, err.Error())
		return
	}
	pkg.OK(c, a)
}

func (h *Handlers) AdminCrawlFailures(c *gin.Context) {
	page, size := pkg.PageParams(c)
	var total int64
	var list []model.CrawlFailure
	h.DB.Model(&model.CrawlFailure{}).Count(&total)
	h.DB.Order("id DESC").Offset((page - 1) * size).Limit(size).Find(&list)
	pkg.OK(c, pkg.PageResult{List: list, Total: total, Page: page, PageSize: size})
}