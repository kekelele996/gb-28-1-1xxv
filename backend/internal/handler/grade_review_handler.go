package handler

import (
	"fmt"
	"log/slog"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson/primitive"

	"github.com/onlineexam/onlineexam/internal/constants"
	"github.com/onlineexam/onlineexam/internal/dto"
	"github.com/onlineexam/onlineexam/internal/middleware"
	"github.com/onlineexam/onlineexam/internal/service"
	"github.com/onlineexam/onlineexam/internal/util"
)

// GradeReviewHandler 成绩复核闭环 HTTP 处理器：
// 学生申请复核 / 教师处理复核 / 教师待复核列表。
type GradeReviewHandler struct {
	svc    *service.GradeReviewService
	logger *slog.Logger
}

// NewGradeReviewHandler 构造成绩复核处理器。
func NewGradeReviewHandler(svc *service.GradeReviewService, logger *slog.Logger) *GradeReviewHandler {
	return &GradeReviewHandler{svc: svc, logger: logger}
}

// actorFromContext 从 JWT 与请求上下文组装复核操作人（审计需要）。
func actorFromContext(c *gin.Context) service.ReviewActor {
	name := ""
	if claims := middleware.GetClaims(c); claims != nil {
		name = claims.Name
	}
	return service.ReviewActor{
		UserID:    middleware.GetUserID(c),
		Name:      name,
		IP:        c.ClientIP(),
		RequestID: middleware.GetRequestID(c),
	}
}

// parseRecordID 解析路径参数 id。
func parseRecordID(c *gin.Context) (primitive.ObjectID, bool) {
	id, err := primitive.ObjectIDFromHex(c.Param("id"))
	if err != nil {
		Error(c, util.NewAppError(constants.CodeBadRequest, "成绩复核模块：id 参数非法"))
		return primitive.NilObjectID, false
	}
	return id, true
}

// Review POST /exam-records/:id/review 的角色分发入口：
// 学生 → 发起复核申请；教师/管理员 → 处理复核。
// 具体归属、24h 窗口、仅一次、pending 锁定、仅改主观题等规则均在 service 层二次校验。
func (h *GradeReviewHandler) Review(c *gin.Context) {
	switch middleware.GetRole(c) {
	case constants.RoleStudent:
		h.RequestReview(c)
	case constants.RoleTeacher, constants.RoleAdmin:
		h.CompleteReview(c)
	default:
		Error(c, util.NewAppError(constants.CodeUnauthorized, constants.MsgUnauthorized))
	}
}

// RequestReview 学生对本人已批改成绩发起复核（发布后 24h 内，仅一次，理由必填）。
func (h *GradeReviewHandler) RequestReview(c *gin.Context) {
	id, ok := parseRecordID(c)
	if !ok {
		return
	}
	var req dto.RequestReviewRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		Error(c, util.WrapAppError(constants.CodeValidationFailed, fmt.Sprintf("成绩复核模块：申请参数校验失败（字段 reason）（%s）", err.Error()), err))
		return
	}
	rec, err := h.svc.RequestReview(c.Request.Context(), id, req.Reason, actorFromContext(c))
	if err != nil {
		Error(c, err)
		return
	}
	SuccessMessage(c, constants.MsgReviewRequestSuccess, dto.ToRecordResponse(rec))
}

// CompleteReview 教师处理复核（仅改主观题，重算总分与及格结果，意见+审计一次落盘）。
func (h *GradeReviewHandler) CompleteReview(c *gin.Context) {
	id, ok := parseRecordID(c)
	if !ok {
		return
	}
	var req dto.CompleteReviewRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		Error(c, util.WrapAppError(constants.CodeValidationFailed, fmt.Sprintf("成绩复核模块：处理参数校验失败（字段 teacher_opinion/grades/result）（%s）", err.Error()), err))
		return
	}
	rec, err := h.svc.CompleteReview(c.Request.Context(), id, req, actorFromContext(c))
	if err != nil {
		Error(c, err)
		return
	}
	SuccessMessage(c, constants.MsgReviewHandleSuccess, dto.ToRecordResponse(rec))
}

// ListPending 教师分页查询待复核记录。
func (h *GradeReviewHandler) ListPending(c *gin.Context) {
	page := util.GetPageParams(c, 20)
	list, total, err := h.svc.ListPending(c.Request.Context(), page.Page, page.PageSize)
	if err != nil {
		Error(c, err)
		return
	}
	items := make([]dto.RecordResponse, 0, len(list))
	for _, r := range list {
		items = append(items, dto.ToRecordResponse(r))
	}
	PageResult(c, items, total, page.Page, page.PageSize)
}
