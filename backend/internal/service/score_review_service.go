package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"

	"github.com/onlineexam/onlineexam/internal/constants"
	"github.com/onlineexam/onlineexam/internal/dto"
	"github.com/onlineexam/onlineexam/internal/model"
	"github.com/onlineexam/onlineexam/internal/repository"
	"github.com/onlineexam/onlineexam/internal/util"
)

// ReviewAuditSink 系统级审计写入（audit_logs 集合，管理员审计页面可见）。
// 与内嵌在成绩文档中的审计轨迹不同，这是 best-effort 的独立写入：
// 成绩、复核记录与内嵌审计已随单文档原子落盘，此写失败不回滚业务数据。
type ReviewAuditSink interface {
	WriteReviewAudit(ctx context.Context, e ReviewAuditEvent)
}

// ReviewAuditEvent 系统级复核审计事件。
type ReviewAuditEvent struct {
	OperatorID   primitive.ObjectID
	OperatorName string
	Role         string
	Action       string
	RecordID     string
	Detail       string
}

// ScoreReviewService 成绩复核服务。
// 闭环：学生在成绩发布后 24h 内对本人已批改记录发起一次复核（理由必填）→ 待复核期间成绩锁定 →
// 教师复核只能改主观题，任一改动都重算总分并按原及格线更新及格结果 →
// 复核意见、修改前后分数与审计轨迹随成绩同一文档一次落盘（乐观锁条件写）→ 学生回读结果。
type ScoreReviewService struct {
	recordRepo repository.ExamRecordRepository
	reviewRepo repository.ScoreReviewRepository
	auditSink  ReviewAuditSink
	logger     *slog.Logger
}

// NewScoreReviewService 构造成绩复核服务。auditSink 可为 nil（不写系统级审计）。
func NewScoreReviewService(recordRepo repository.ExamRecordRepository, reviewRepo repository.ScoreReviewRepository, auditSink ReviewAuditSink, logger *slog.Logger) *ScoreReviewService {
	return &ScoreReviewService{recordRepo: recordRepo, reviewRepo: reviewRepo, auditSink: auditSink, logger: logger}
}

func (s *ScoreReviewService) emitAudit(ctx context.Context, e ReviewAuditEvent) {
	if s.auditSink != nil {
		s.auditSink.WriteReviewAudit(ctx, e)
	}
}

// Apply 学生发起成绩复核。
func (s *ScoreReviewService) Apply(ctx context.Context, recordID, studentID primitive.ObjectID, studentName, reason string) (*model.ExamRecord, error) {
	reason = strings.TrimSpace(reason)
	if reason == "" {
		return nil, util.NewAppError(constants.CodeBadRequest, constants.MsgReviewReasonEmpty)
	}

	rec, err := s.recordRepo.FindByID(ctx, recordID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, util.NewAppError(constants.CodeRecordNotFound, fmt.Sprintf(constants.MsgRecordNotFound, recordID.Hex()))
		}
		return nil, fmt.Errorf("score review service apply find: %w", err)
	}
	// 只能对本人记录发起复核。
	if rec.StudentID != studentID {
		return nil, util.NewAppError(constants.CodeForbidden, fmt.Sprintf(constants.MsgReviewNotOwner, constants.RoleStudent, recordID.Hex()))
	}
	// 成绩必须已发布（已批改完成）。
	if rec.Status != constants.RecordStatusGraded || rec.GradedAt == nil {
		return nil, util.NewAppError(constants.CodeReviewNotGraded, fmt.Sprintf(constants.MsgReviewNotGraded, recordID.Hex(), rec.Status))
	}
	// 只能发起一次复核（重复提交拒绝；状态机：无复核 → pending）。
	if rec.Review != nil {
		return nil, util.NewAppError(constants.CodeReviewDuplicate, fmt.Sprintf(constants.MsgReviewDuplicate, recordID.Hex(), rec.Review.Status))
	}
	// 成绩发布后 24 小时窗口。
	now := time.Now()
	expiresAt := rec.GradedAt.Add(constants.ScoreReviewWindowHours * time.Hour)
	if now.After(expiresAt) {
		return nil, util.NewAppError(constants.CodeReviewWindowExpired, fmt.Sprintf(constants.MsgReviewWindowExpired, recordID.Hex(), util.FormatDateTime(expiresAt)))
	}

	passedBefore := rec.Passed
	appliedAt := now
	rec.Review = &model.ScoreReview{
		RecordID:         rec.ID,
		ExamID:           rec.ExamID,
		StudentID:        rec.StudentID,
		ExamTitle:        rec.ExamTitle,
		StudentName:      rec.StudentName,
		Reason:           reason,
		Status:           constants.ReviewStatusPending, // 申请即进入待复核，成绩锁定
		FinalScoreBefore: rec.FinalScore,
		FinalScoreAfter:  rec.FinalScore,
		PassedBefore:     &passedBefore,
		PassScore:        rec.PassScore,
		Changes:          []model.ScoreReviewChange{},
		AppliedAt:        &appliedAt,
		ExpiresAt:        expiresAt,
		AuditTrail: []model.ReviewAuditEntry{{
			Action:       constants.AuditActionReviewApply,
			OperatorID:   studentID,
			OperatorName: studentName,
			Role:         constants.RoleStudent,
			Comment:      reason,
			CreatedAt:    now,
		}},
	}
	rec.UpdatedAt = now
	rec.Version++
	if err := s.recordRepo.SaveIfVersion(ctx, rec, rec.Version-1); err != nil {
		// 重复提交/并发冲突/写入失败：复核记录、成绩与审计均不写入，保持原样。
		return nil, s.mapWriteErr(recordID, err, "score review service apply")
	}
	s.logger.Info(constants.LogReviewApplied, "record_id", rec.ID.Hex(), "exam_id", rec.ExamID.Hex(), "student", studentName, "final_score", rec.FinalScore)
	s.emitAudit(ctx, ReviewAuditEvent{
		OperatorID: studentID, OperatorName: studentName, Role: constants.RoleStudent,
		Action: constants.AuditActionReviewApply, RecordID: rec.ID.Hex(), Detail: reason,
	})
	return rec, nil
}

// Complete 教师复核裁定：只能改主观题；任一分值变化重算总分并按原及格线更新结果。
func (s *ScoreReviewService) Complete(ctx context.Context, recordID, teacherID primitive.ObjectID, teacherName string, req dto.CompleteReviewRequest) (*model.ExamRecord, error) {
	comment := strings.TrimSpace(req.TeacherComment)
	if comment == "" {
		return nil, util.NewAppError(constants.CodeReviewDecisionErr, constants.MsgReviewCommentEmpty)
	}
	if req.Decision != constants.ReviewStatusAdjusted && req.Decision != constants.ReviewStatusRejected {
		return nil, util.NewAppError(constants.CodeReviewDecisionErr, constants.MsgReviewDecisionBad)
	}

	rec, err := s.recordRepo.FindByID(ctx, recordID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, util.NewAppError(constants.CodeRecordNotFound, fmt.Sprintf(constants.MsgRecordNotFound, recordID.Hex()))
		}
		return nil, fmt.Errorf("score review service decide find: %w", err)
	}
	if rec.Review == nil {
		return nil, util.NewAppError(constants.CodeReviewNotFound, fmt.Sprintf(constants.MsgReviewNotFound, recordID.Hex()))
	}
	if rec.Review.Status != constants.ReviewStatusPending {
		// 并发复核：已裁定的复核不能再次裁定。
		return nil, util.NewAppError(constants.CodeReviewNotPending, fmt.Sprintf(constants.MsgReviewNotPending, recordID.Hex(), rec.Review.Status))
	}

	now := time.Now()
	changes := make([]model.ScoreReviewChange, 0)
	scoreChanged := false

	if req.Decision == constants.ReviewStatusAdjusted {
		if len(req.Items) == 0 {
			return nil, util.NewAppError(constants.CodeReviewDecisionErr, "成绩复核模块：裁定为已调整时 items 至少包含一项主观题改分")
		}
		itemMap := make(map[string]dto.ReviewGradeItem, len(req.Items))
		for _, it := range req.Items {
			itemMap[it.QuestionID] = it
		}
		for i := range rec.Questions {
			q := &rec.Questions[i]
			it, ok := itemMap[q.QuestionID.Hex()]
			if !ok {
				continue
			}
			// 复核只能改主观题。
			if constants.IsObjectiveQuestion(q.Type) {
				return nil, util.NewAppError(constants.CodeReviewObjectiveOnly, fmt.Sprintf(constants.MsgReviewObjectiveOnly, q.QuestionID.Hex(), q.Type))
			}
			if it.Score < 0 || it.Score > q.Score {
				return nil, util.NewAppError(constants.CodeBadRequest, fmt.Sprintf(constants.MsgReviewScoreRange, q.QuestionID.Hex(), it.Score, q.Score))
			}
			beforeScore := q.GotScore
			beforeComment := q.Comment
			q.SubjectiveScore = it.Score
			q.GotScore = it.Score
			q.Marked = true
			if strings.TrimSpace(it.Comment) != "" {
				q.Comment = it.Comment
			}
			if it.Score != beforeScore {
				scoreChanged = true
			}
			changes = append(changes, model.ScoreReviewChange{
				QuestionID:    q.QuestionID,
				Content:       q.Content,
				QuestionType:  q.Type,
				ScoreBefore:   beforeScore,
				ScoreAfter:    it.Score,
				CommentBefore: beforeComment,
				CommentAfter:  q.Comment,
			})
		}
		// 提交的 question_id 必须都能在答卷中找到。
		if len(changes) != len(itemMap) {
			for _, it := range req.Items {
				found := false
				for i := range rec.Questions {
					if rec.Questions[i].QuestionID.Hex() == it.QuestionID {
						found = true
						break
					}
				}
				if !found {
					return nil, util.NewAppError(constants.CodeBadRequest, fmt.Sprintf(constants.MsgReviewItemInvalid, it.QuestionID))
				}
			}
		}
		// 任一分值变化都重新计算总分。
		if scoreChanged {
			var subjectiveTotal float64
			for i := range rec.Questions {
				if !constants.IsObjectiveQuestion(rec.Questions[i].Type) {
					subjectiveTotal += rec.Questions[i].GotScore
				}
			}
			rec.SubjectiveScore = subjectiveTotal
			rec.FinalScore = rec.ObjectiveScore + subjectiveTotal
		}
	}

	// 裁定为“已调整”时必须存在至少一项实际分值变化；无变化应选择维持原判。
	if req.Decision == constants.ReviewStatusAdjusted && !scoreChanged {
		return nil, util.NewAppError(constants.CodeReviewDecisionErr, "成绩复核模块：任一分值均未变化，请选择“维持原判”或填写改分")
	}
	finalDecision := req.Decision

	// 按原及格线更新及格结果。
	rec.Passed = rec.PassScore > 0 && rec.FinalScore >= rec.PassScore
	decidedAt := now
	passedAfter := rec.Passed
	review := rec.Review
	review.Status = finalDecision
	review.FinalScoreAfter = rec.FinalScore
	review.PassedAfter = &passedAfter
	review.Changes = changes
	review.TeacherID = &teacherID
	review.TeacherName = teacherName
	review.TeacherComment = comment
	review.DecidedAt = &decidedAt
	review.AuditTrail = append(review.AuditTrail, model.ReviewAuditEntry{
		Action:       constants.AuditActionReviewDecide,
		OperatorID:   teacherID,
		OperatorName: teacherName,
		Role:         constants.RoleTeacher,
		Comment:      comment,
		CreatedAt:    now,
	})
	rec.UpdatedAt = now
	rec.Version++
	// 复核意见、修改前后分数、审计记录随成绩同一文档一次落盘；并发复核或写入失败则全部保持原样。
	if err := s.recordRepo.SaveIfVersion(ctx, rec, rec.Version-1); err != nil {
		return nil, s.mapWriteErr(recordID, err, "score review service decide")
	}
	passedBefore := review.PassedBefore
	s.logger.Info(constants.LogReviewDecided, "record_id", rec.ID.Hex(), "decision", finalDecision,
		"score_before", review.FinalScoreBefore, "score_after", review.FinalScoreAfter,
		"passed_before", passedBefore, "passed_after", passedAfter, "teacher", teacherName)
	s.emitAudit(ctx, ReviewAuditEvent{
		OperatorID: teacherID, OperatorName: teacherName, Role: constants.RoleTeacher,
		Action: constants.AuditActionReviewDecide, RecordID: rec.ID.Hex(),
		Detail: fmt.Sprintf("decision=%s score %g→%g comment=%s", finalDecision, review.FinalScoreBefore, review.FinalScoreAfter, comment),
	})
	return rec, nil
}

// GetForViewer 按角色读取复核详情：学生只能看本人，教师/管理员不限。
func (s *ScoreReviewService) GetForViewer(ctx context.Context, recordID, viewerID primitive.ObjectID, role string) (*model.ExamRecord, error) {
	rec, err := s.recordRepo.FindByID(ctx, recordID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, util.NewAppError(constants.CodeRecordNotFound, fmt.Sprintf(constants.MsgRecordNotFound, recordID.Hex()))
		}
		return nil, fmt.Errorf("score review service get: %w", err)
	}
	if role == constants.RoleStudent && rec.StudentID != viewerID {
		return nil, util.NewAppError(constants.CodeForbidden, fmt.Sprintf(constants.MsgReviewNotOwner, role, recordID.Hex()))
	}
	return rec, nil
}

// ListPending 教师/管理员查询待复核（可按全部/待处理状态过滤）。
func (s *ScoreReviewService) ListPending(ctx context.Context, statuses []string, page, pageSize int64) ([]*model.ExamRecord, int64, error) {
	if len(statuses) == 0 {
		statuses = []string{constants.ReviewStatusPending, constants.ReviewStatusAdjusted, constants.ReviewStatusRejected}
	}
	list, total, err := s.reviewRepo.ListByStatus(ctx, statuses, page, pageSize)
	if err != nil {
		return nil, 0, fmt.Errorf("score review service list pending: %w", err)
	}
	return list, total, nil
}

// ListMine 学生查询自己的复核记录（回读申请、复核状态与结果）。
func (s *ScoreReviewService) ListMine(ctx context.Context, studentID primitive.ObjectID, statuses []string, page, pageSize int64) ([]*model.ExamRecord, int64, error) {
	list, total, err := s.reviewRepo.ListByStudent(ctx, studentID, statuses, page, pageSize)
	if err != nil {
		return nil, 0, fmt.Errorf("score review service list mine: %w", err)
	}
	return list, total, nil
}

func (s *ScoreReviewService) mapWriteErr(recordID primitive.ObjectID, err error, wrapMsg string) error {
	if errors.Is(err, repository.ErrConflict) {
		return util.NewAppError(constants.CodeRecordVersion, fmt.Sprintf(constants.MsgRecordVersion, recordID.Hex()))
	}
	if errors.Is(err, repository.ErrNotFound) {
		return util.NewAppError(constants.CodeRecordNotFound, fmt.Sprintf(constants.MsgRecordNotFound, recordID.Hex()))
	}
	return fmt.Errorf("%s: %w", wrapMsg, err)
}
