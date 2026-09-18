package router

import (
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/onlineexam/onlineexam/internal/handler"
)

// TestExamRecordRoutesRegister 成绩复核相关路由必须能在同一棵 Gin 路由树上无冲突注册
//（历史上同一路径在学生/教师两个 group 重复注册会在启动时 panic）。
func TestExamRecordRoutesRegister(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	g := engine.Group("/api/v1")

	rh := handler.NewExamRecordHandler(nil, nil)
	gh := handler.NewGradeReviewHandler(nil, nil)

	// 不应 panic（重复路径或静态/通配段冲突会在注册阶段 panic）。
	RegisterExamRecordRoutes(g, rh, gh)

	wantPaths := map[string]bool{
		"POST /api/v1/exam-records/:id/review":     false,
		"GET /api/v1/reviews/pending":              false,
		"POST /api/v1/exam-records/:id/grade":      false,
		"GET /api/v1/exam-records/:id":             false,
	}
	for _, ri := range engine.Routes() {
		key := ri.Method + " " + ri.Path
		if _, ok := wantPaths[key]; ok {
			wantPaths[key] = true
		}
	}
	for p, found := range wantPaths {
		if !found {
			t.Errorf("路由未注册: %s", p)
		}
	}
}
