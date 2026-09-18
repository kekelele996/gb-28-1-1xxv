package router

import (
	"github.com/gin-gonic/gin"

	"github.com/onlineexam/onlineexam/internal/constants"
	"github.com/onlineexam/onlineexam/internal/handler"
	"github.com/onlineexam/onlineexam/internal/middleware"
)

// RegisterExamRecordRoutes 考试记录模块路由（含成绩复核闭环）。
func RegisterExamRecordRoutes(g *gin.RouterGroup, h *handler.ExamRecordHandler, rh *handler.GradeReviewHandler) {
	// 学生：开始考试 / 我的记录 / 提交
	student := g.Group("/exam-records", middleware.RequireRoles(constants.RoleStudent))
	{
		student.POST("/:id/start", h.Start)
		student.GET("/mine", h.ListMine)
		student.POST("/:id/submit", h.Submit)
	}

	// 教师/管理员：按试卷查记录、批改、复核处理
	teacher := g.Group("/exam-records", middleware.RequireRoles(constants.RoleTeacher, constants.RoleAdmin))
	{
		teacher.GET("/exam/:id", h.ListByExam)
		teacher.POST("/:id/grade", h.Grade)
		teacher.POST("/:id/auto-submit", h.AutoSubmit)
	}

	// 成绩复核闭环：
	//  - 同一 REST 路径 POST /exam-records/:id/review，由处理器按角色分发
	//    （学生→发起申请；教师/管理员→处理），service 层再做归属/窗口/锁定二次校验。
	//  - GET /reviews/pending 教师待复核列表（独立前缀，避免与 /exam-records/:id 通配段冲突）。
	g.POST("/exam-records/:id/review", rh.Review)
	g.GET("/reviews/pending", middleware.RequireRoles(constants.RoleTeacher, constants.RoleAdmin), rh.ListPending)

	g.GET("/exams/:id/records", middleware.RequireRoles(constants.RoleTeacher, constants.RoleAdmin), h.ListByExam)
	g.GET("/exams/:id/report", middleware.RequireRoles(constants.RoleTeacher, constants.RoleAdmin), h.Report)
	g.GET("/exam-records/:id", h.Get)
}
