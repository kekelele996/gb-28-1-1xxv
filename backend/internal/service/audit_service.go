// Package service 实现业务逻辑，依赖注入 repository 接口。
package service

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"

	"github.com/onlineexam/onlineexam/internal/constants"
	"github.com/onlineexam/onlineexam/internal/model"
	"github.com/onlineexam/onlineexam/internal/repository"
)

// AuditService 审计日志服务。
type AuditService struct {
	repo   repository.AuditRepository
	logger *slog.Logger
}

// NewAuditService 构造审计服务。
func NewAuditService(repo repository.AuditRepository, logger *slog.Logger) *AuditService {
	return &AuditService{repo: repo, logger: logger}
}

// Create 写入一条审计日志（中间件与 service 埋点共用）。
func (s *AuditService) Create(ctx context.Context, entry *model.AuditLog) error {
	entry.CreatedAt = time.Now()
	if entry.ID.IsZero() {
		entry.ID = primitive.NewObjectID()
	}
	if err := s.repo.Create(ctx, entry); err != nil {
		s.logger.Error("写入审计日志失败", "module", entry.Module, "action", entry.Action, "error", err.Error())
		return fmt.Errorf("audit service create: %w", err)
	}
	s.logger.Info(constants.LogAuditCreated, "module", entry.Module, "action", entry.Action, "user", entry.Username)
	return nil
}

// List 分页查询审计日志（审计页面复用）。
func (s *AuditService) List(ctx context.Context, filter bson.M, page, pageSize int64) ([]*model.AuditLog, int64, error) {
	list, total, err := s.repo.List(ctx, filter, page, pageSize)
	if err != nil {
		return nil, 0, fmt.Errorf("audit service list: %w", err)
	}
	return list, total, nil
}

// WriteReviewAudit 实现 service.ReviewAuditSink：成绩复核关键动作写入系统审计集合。
// best-effort：成绩与内嵌审计已随单文档原子落盘，此处失败仅告警，不影响复核结果。
func (s *AuditService) WriteReviewAudit(ctx context.Context, e ReviewAuditEvent) {
	entry := &model.AuditLog{
		ID:        primitive.NewObjectID(),
		UserID:    e.OperatorID,
		Username:  e.OperatorName,
		Role:      e.Role,
		Module:    "score-review",
		Action:    e.Action,
		Method:    "POST",
		Path:      "/api/v1/exam-records/" + e.RecordID + "/review",
		Detail:    e.Detail,
		CreatedAt: time.Now(),
	}
	if err := s.repo.Create(ctx, entry); err != nil {
		s.logger.Warn("成绩复核系统审计写入失败", "record_id", e.RecordID, "action", e.Action, "error", err.Error())
	}
}
