package router

import (
	"github.com/gin-gonic/gin"

	"github.com/onlineexam/onlineexam/internal/constants"
	"github.com/onlineexam/onlineexam/internal/handler"
	"github.com/onlineexam/onlineexam/internal/middleware"
)

// RegisterScoreReviewRoutes 成绩复核模块路由。
// 为避免与 /exam-records/:id 同级静态段/通配符冲突，复核集合路由使用 /score-reviews 前缀，
// 挂在具体答卷上的动作用 /exam-records/:id/review（GET 读、POST 申请、POST/decide 教师裁定）。
func RegisterScoreReviewRoutes(g *gin.RouterGroup, h *handler.ScoreReviewHandler) {
	// 学生：发起复核、查询本人复核列表
	student := g.Group("", middleware.RequireRoles(constants.RoleStudent))
	{
		student.POST("/exam-records/:id/review", h.Apply)
		student.GET("/score-reviews/mine", h.ListMine)
	}

	// 教师/管理员：复核列表、复核裁定
	teacher := g.Group("", middleware.RequireRoles(constants.RoleTeacher, constants.RoleAdmin))
	{
		teacher.GET("/score-reviews", h.ListPending)
		teacher.POST("/exam-records/:id/review/decide", h.Decide)
	}

	// 已登录：按角色读取复核详情（service 层校验学生只能读本人）
	g.GET("/exam-records/:id/review", middleware.RequireRoles(
		constants.RoleStudent, constants.RoleTeacher, constants.RoleAdmin), h.Get)
}
