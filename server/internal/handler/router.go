package handler

import (
	"gongkao/internal/middleware"
	"gongkao/internal/pkg"

	"github.com/gin-gonic/gin"
)

func Register(r *gin.Engine, h *Handlers) {
	r.GET("/health", h.Health)

	v1 := r.Group("/api/v1")
	{
		v1.POST("/auth/wechat", h.WechatLogin)
		v1.POST("/auth/refresh", h.Refresh)

		v1.GET("/announcements", h.ListAnnouncements)
		v1.GET("/announcements/:id", h.GetAnnouncement)

		v1.GET("/questions/categories", h.ListCategories)

		v1.GET("/affairs", h.ListAffairs)
		v1.GET("/affairs/:id", middleware.OptionalAuth(h.JWT), h.GetAffair)

		me := v1.Group("/me", middleware.Auth(h.JWT, pkg.TokenAccess))
		{
			me.GET("", h.Me)
			me.PATCH("", h.UpdateMe)
			me.GET("/exams", h.ListMyExams)
			me.POST("/exams", h.JoinExam)
			me.DELETE("/exams/:announcementId", h.LeaveExam)
			me.GET("/todos", h.ListTodos)
			me.PATCH("/todos/:id", h.PatchTodo)
			me.GET("/wrong-questions", h.ListWrong)
			me.DELETE("/wrong-questions/:questionId", h.DeleteWrong)
			me.GET("/practice/stats", h.PracticeStats)
			me.GET("/affairs/favorites", h.ListFavorites)
			me.POST("/affairs/favorites", h.FavoriteAffair)
			me.DELETE("/affairs/favorites/:id", h.UnfavoriteAffair)
		}

		prac := v1.Group("/practice", middleware.Auth(h.JWT, pkg.TokenAccess))
		{
			prac.POST("/sessions", h.StartPractice)
			prac.POST("/submit", h.SubmitPractice)
		}

		admin := v1.Group("/admin")
		{
			admin.POST("/login", h.AdminLogin)
			authed := admin.Group("", middleware.Auth(h.JWT, pkg.TokenAdmin))
			{
				authed.GET("/announcements", h.AdminListAnnouncements)
				authed.POST("/announcements", h.AdminCreateAnnouncement)
				authed.PATCH("/announcements/:id", h.AdminUpdateAnnouncement)
				authed.POST("/questions/import", h.AdminImportQuestions)
				authed.POST("/affairs", h.AdminCreateAffair)
				authed.PATCH("/affairs/:id", h.AdminUpdateAffair)
				authed.GET("/crawl-failures", h.AdminCrawlFailures)
			}
		}
	}
}