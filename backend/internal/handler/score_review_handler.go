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

// ScoreReviewHandler 成绩复核 HTTP 处理器。
type ScoreReviewHandler struct {
	svc    *service.ScoreReviewService
	logger *slog.Logger
}

// NewScoreReviewHandler 构造成绩复核处理器。
func NewScoreReviewHandler(svc *service.ScoreReviewService, logger *slog.Logger) *ScoreReviewHandler {
	return &ScoreReviewHandler{svc: svc, logger: logger}
}

// Apply 学生对本人已批改记录发起一次成绩复核（成绩发布后 24h 内、理由必填）。
func (h *ScoreReviewHandler) Apply(c *gin.Context) {
	id, err := primitive.ObjectIDFromHex(c.Param("id"))
	if err != nil {
		Error(c, util.NewAppError(constants.CodeBadRequest, "成绩复核模块：recordId 参数非法"))
		return
	}
	var req dto.CreateReviewRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		Error(c, util.WrapAppError(constants.CodeValidationFailed, fmt.Sprintf("成绩复核模块：申请参数校验失败（字段 reason）（%s）", err.Error()), err))
		return
	}
	claims := middleware.GetClaims(c)
	name := ""
	if claims != nil {
		name = claims.Name
	}
	rec, err := h.svc.Apply(c.Request.Context(), id, middleware.GetUserID(c), name, req.Reason)
	if err != nil {
		Error(c, err)
		return
	}
	SuccessMessage(c, constants.MsgReviewApplySuccess, dto.ToRecordResponse(rec))
}

// Get 按角色读取复核/答卷详情（学生限本人）。
func (h *ScoreReviewHandler) Get(c *gin.Context) {
	id, err := primitive.ObjectIDFromHex(c.Param("id"))
	if err != nil {
		Error(c, util.NewAppError(constants.CodeBadRequest, "成绩复核模块：recordId 参数非法"))
		return
	}
	rec, err := h.svc.GetForViewer(c.Request.Context(), id, middleware.GetUserID(c), middleware.GetRole(c))
	if err != nil {
		Error(c, err)
		return
	}
	Success(c, dto.ToRecordResponse(rec))
}

// Decide 教师复核裁定（只能改主观题；重算总分、按原及格线更新结果，一次落盘）。
func (h *ScoreReviewHandler) Decide(c *gin.Context) {
	id, err := primitive.ObjectIDFromHex(c.Param("id"))
	if err != nil {
		Error(c, util.NewAppError(constants.CodeBadRequest, "成绩复核模块：recordId 参数非法"))
		return
	}
	var req dto.CompleteReviewRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		Error(c, util.WrapAppError(constants.CodeValidationFailed, fmt.Sprintf("成绩复核模块：复核裁定参数校验失败（字段 decision/teacher_comment/items）（%s）", err.Error()), err))
		return
	}
	claims := middleware.GetClaims(c)
	name := ""
	if claims != nil {
		name = claims.Name
	}
	rec, err := h.svc.Complete(c.Request.Context(), id, middleware.GetUserID(c), name, req)
	if err != nil {
		Error(c, err)
		return
	}
	SuccessMessage(c, constants.MsgReviewDecideSuccess, dto.ToRecordResponse(rec))
}

// ListPending 教师/管理员复核列表（默认全部，可按 status 过滤，pending 为待复核）。
func (h *ScoreReviewHandler) ListPending(c *gin.Context) {
	statuses := []string{}
	if s := c.Query("status"); s != "" {
		statuses = []string{s}
	}
	page := util.GetPageParams(c, 20)
	list, total, err := h.svc.ListPending(c.Request.Context(), statuses, page.Page, page.PageSize)
	if err != nil {
		Error(c, err)
		return
	}
	items := make([]dto.ScoreReviewResponse, 0, len(list))
	for _, r := range list {
		if rv := dto.ToScoreReviewResponse(r.Review); rv != nil {
			items = append(items, *rv)
		}
	}
	PageResult(c, items, total, page.Page, page.PageSize)
}

// ListMine 学生本人复核列表（回读申请、复核状态与结果）。
func (h *ScoreReviewHandler) ListMine(c *gin.Context) {
	statuses := []string{}
	if s := c.Query("status"); s != "" {
		statuses = []string{s}
	}
	page := util.GetPageParams(c, 20)
	list, total, err := h.svc.ListMine(c.Request.Context(), middleware.GetUserID(c), statuses, page.Page, page.PageSize)
	if err != nil {
		Error(c, err)
		return
	}
	items := make([]dto.ScoreReviewResponse, 0, len(list))
	for _, r := range list {
		if rv := dto.ToScoreReviewResponse(r.Review); rv != nil {
			items = append(items, *rv)
		}
	}
	PageResult(c, items, total, page.Page, page.PageSize)
}
