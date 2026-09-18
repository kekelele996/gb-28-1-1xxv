package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"

	"github.com/onlineexam/onlineexam/internal/constants"
	"github.com/onlineexam/onlineexam/internal/dto"
	"github.com/onlineexam/onlineexam/internal/model"
	"github.com/onlineexam/onlineexam/internal/repository"
	"github.com/onlineexam/onlineexam/internal/util"
)

// GradeReviewService 成绩复核闭环服务。
//
// 业务规则：
//  1. 学生仅能在成绩发布（record.graded_at）后 24h 内，对本人已批改（graded）记录发起一次复核，
//     复核理由 reason 必填；
//  2. 待复核（pending）期间成绩锁定：普通批改接口与复核处理互斥；
//  3. 教师复核只能调整主观题（fill/short），客观题分值不可变；
//  4. 任一分值变化都重算主观题总分、总分 final_score，并按原及格线 pass_score 更新 passed；
//  5. 复核意见、修改前后题分快照、总分快照与审计记录内嵌于同一文档一次原子落盘；
//  6. 重复提交、并发复核或写入失败时，原子条件替换不命中，记录、成绩和审计保持原样。
type GradeReviewService struct {
	repo   repository.ExamRecordRepository
	logger *slog.Logger
}

// NewGradeReviewService 构造成绩复核服务。
func NewGradeReviewService(repo repository.ExamRecordRepository, logger *slog.Logger) *GradeReviewService {
	return &GradeReviewService{repo: repo, logger: logger}
}

// ReviewActor 操作人上下文（HTTP handler 从 JWT 与请求上下文组装）。
type ReviewActor struct {
	UserID    primitive.ObjectID
	Name      string
	IP        string
	RequestID string
}

// RequestReview 学生发起成绩复核。
func (s *GradeReviewService) RequestReview(ctx context.Context, recordID primitive.ObjectID, reason string, actor ReviewActor) (*model.ExamRecord, error) {
	rec, err := s.repo.FindByID(ctx, recordID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, util.NewAppError(constants.CodeRecordNotFound, fmt.Sprintf(constants.MsgRecordNotFound, recordID.Hex()))
		}
		return nil, fmt.Errorf("grade review service request find: %w", err)
	}
	// 仅本人可对自己的成绩发起复核。
	if rec.StudentID != actor.UserID {
		return nil, util.NewAppError(constants.CodeReviewNotOwner, fmt.Sprintf(constants.MsgReviewNotOwner, recordID.Hex()))
	}
	// 必须为已批改发布的成绩。
	if rec.Status != constants.RecordStatusGraded || rec.GradedAt == nil {
		return nil, util.NewAppError(constants.CodeRecordStatusErr, fmt.Sprintf(constants.MsgReviewNotGraded, recordID.Hex()))
	}
	// 每条成绩仅可复核一次（pending/adjusted/rejected 都视为已申请）。
	if rec.Review != nil {
		return nil, util.NewAppError(constants.CodeReviewDuplicate, fmt.Sprintf(constants.MsgReviewDuplicate, recordID.Hex(), rec.Review.Status))
	}
	// 成绩发布后 24 小时窗口。
	if time.Since(*rec.GradedAt) > constants.ReviewWindow {
		s.logger.Warn(constants.LogReviewRejected, "record_id", recordID.Hex(), "reason_code", constants.CodeReviewWindowClose, "student", rec.StudentName)
		return nil, util.NewAppError(constants.CodeReviewWindowClose, fmt.Sprintf(constants.MsgReviewWindowClose, recordID.Hex()))
	}

	now := time.Now()
	review := &model.GradeReview{
		Status:       constants.ReviewStatusPending,
		Reason:       reason,
		BeforeScores: snapshotSubjectiveScores(rec.Questions),
		RequestedAt:  now,
		CreatedAt:    now,
		UpdatedAt:    now,
		Audit: model.ReviewAudit{
			RequestedBy:           actor.UserID,
			RequestedByName:       actor.Name,
			RequestedAt:           now,
			RequestIP:             actor.IP,
			RequestRequestID:      actor.RequestID,
			BeforeObjectiveScore:  rec.ObjectiveScore,
			BeforeSubjectiveScore: rec.SubjectiveScore,
			BeforeFinalScore:      rec.FinalScore,
			BeforePassed:          rec.Passed,
		},
	}
	rec.Review = review
	rec.UpdatedAt = now

	// 原子条件替换：必须仍是 graded、无复核记录、且 graded_at 未变，
	// 任一条件被并发请求改变则整次写入放弃，记录/成绩/审计保持原样。
	guard := bson.M{
		"status":    constants.RecordStatusGraded,
		"review":    nil,
		"graded_at": rec.GradedAt,
	}
	matched, err := s.repo.ReplaceIf(ctx, rec, guard)
	if err != nil {
		return nil, fmt.Errorf("grade review service request: %w", err)
	}
	if !matched {
		s.logger.Warn(constants.LogReviewRejected, "record_id", recordID.Hex(), "reason_code", constants.CodeReviewConflict, "student", rec.StudentName)
		return nil, util.NewAppError(constants.CodeReviewConflict, fmt.Sprintf(constants.MsgReviewConflict, recordID.Hex()))
	}
	s.logger.Info(constants.LogReviewRequested, "record_id", recordID.Hex(), "student", rec.StudentName, "final_score", rec.FinalScore)
	return rec, nil
}

// CompleteReview 教师处理复核：仅改主观题、重算总分与及格结果，复核意见与审计一次落盘。
func (s *GradeReviewService) CompleteReview(ctx context.Context, recordID primitive.ObjectID, req dto.CompleteReviewRequest, actor ReviewActor) (*model.ExamRecord, error) {
	rec, err := s.repo.FindByID(ctx, recordID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, util.NewAppError(constants.CodeRecordNotFound, fmt.Sprintf(constants.MsgRecordNotFound, recordID.Hex()))
		}
		return nil, fmt.Errorf("grade review service complete find: %w", err)
	}
	if rec.Review == nil || rec.Review.Status != constants.ReviewStatusPending {
		return nil, util.NewAppError(constants.CodeReviewNotFound, fmt.Sprintf(constants.MsgReviewNotFound, recordID.Hex()))
	}

	// 仅可改主观题：构建给分索引；逐题校验题型与分值范围，然后应用新分值。
	gradeMap := make(map[string]dto.ReviewGradeItem, len(req.Grades))
	for _, g := range req.Grades {
		gradeMap[g.QuestionID] = g
	}

	// 修改前快照（申请时已固化，再取一次用于逐题变化对比）。
	beforeSubjective := rec.SubjectiveScore
	beforeFinal := rec.FinalScore
	changes := make([]model.ReviewScoreChange, 0, len(req.Grades))

	var subjectiveTotal float64
	for i := range rec.Questions {
		q := &rec.Questions[i]
		if constants.IsObjectiveQuestion(q.Type) {
			// 客观题出现在给分列表中即拒绝（教师复核只能改主观题）。
			if _, ok := gradeMap[q.QuestionID.Hex()]; ok {
				return nil, util.NewAppError(constants.CodeRecordStatusErr, fmt.Sprintf(constants.MsgReviewObjectiveOnly, q.QuestionID.Hex()))
			}
			continue
		}
		if g, ok := gradeMap[q.QuestionID.Hex()]; ok {
			if g.Score < 0 || g.Score > q.Score {
				return nil, util.NewAppError(constants.CodeBadRequest, fmt.Sprintf(constants.MsgReviewScoreRange, q.QuestionID.Hex(), g.Score, q.Score))
			}
			before := q.SubjectiveScore
			q.SubjectiveScore = g.Score
			q.GotScore = g.Score
			q.Marked = true
			if g.Comment != "" {
				q.Comment = g.Comment
			}
			if g.Score > 0 {
				q.Result = constants.AnswerResultPartial
			} else {
				q.Result = constants.AnswerResultWrong
			}
			if before != g.Score {
				changes = append(changes, model.ReviewScoreChange{
					QuestionID: q.QuestionID.Hex(),
					Type:       q.Type,
					Before:     before,
					After:      g.Score,
				})
			}
		}
		subjectiveTotal += q.SubjectiveScore
	}

	// 任一分值变化都重算总分；客观题分不变，并按原及格线更新及格结果。
	rec.SubjectiveScore = subjectiveTotal
	rec.FinalScore = rec.ObjectiveScore + subjectiveTotal
	rec.Passed = rec.PassScore > 0 && rec.FinalScore >= rec.PassScore

	changed := len(changes) > 0 || beforeSubjective != rec.SubjectiveScore || beforeFinal != rec.FinalScore
	status := constants.ReviewStatusRejected
	if changed {
		status = constants.ReviewStatusAdjusted
	}
	// 显式声明的结论必须与分值变化一致，避免审计与成绩不符。
	if req.Result != "" {
		if !constants.IsValidReviewStatus(req.Result) || req.Result == constants.ReviewStatusPending {
			return nil, util.NewAppError(constants.CodeBadRequest, "成绩复核模块：字段 result 仅可为 adjusted 或 rejected")
		}
		if (req.Result == constants.ReviewStatusAdjusted) != changed {
			return nil, util.NewAppError(constants.CodeBadRequest, "成绩复核模块：result 与实际分值变化不一致")
		}
		status = req.Result
	}

	now := time.Now()
	rec.Review.Status = status
	rec.Review.TeacherOpinion = req.Opinion
	rec.Review.AfterScores = snapshotSubjectiveScores(rec.Questions)
	rec.Review.HandledAt = &now
	rec.Review.UpdatedAt = now
	rec.Review.Audit.HandledBy = actor.UserID
	rec.Review.Audit.HandledByName = actor.Name
	rec.Review.Audit.HandledAt = &now
	rec.Review.Audit.HandleIP = actor.IP
	rec.Review.Audit.HandleRequestID = actor.RequestID
	rec.Review.Audit.AfterObjectiveScore = rec.ObjectiveScore
	rec.Review.Audit.AfterSubjectiveScore = rec.SubjectiveScore
	rec.Review.Audit.AfterFinalScore = rec.FinalScore
	rec.Review.Audit.AfterPassed = rec.Passed
	rec.Review.Audit.ScoreChanges = changes
	rec.UpdatedAt = now

	// 原子条件替换：复核必须仍是 pending（防止并发重复处理）。
	guard := bson.M{
		"status":        constants.RecordStatusGraded,
		"review.status": constants.ReviewStatusPending,
	}
	matched, err := s.repo.ReplaceIf(ctx, rec, guard)
	if err != nil {
		return nil, fmt.Errorf("grade review service complete: %w", err)
	}
	if !matched {
		s.logger.Warn(constants.LogReviewRejected, "record_id", recordID.Hex(), "reason_code", constants.CodeReviewConflict, "teacher", actor.Name)
		return nil, util.NewAppError(constants.CodeReviewConflict, fmt.Sprintf(constants.MsgReviewConflict, recordID.Hex()))
	}
	s.logger.Info(constants.LogReviewHandled, "record_id", recordID.Hex(), "status", status, "before_final", beforeFinal, "after_final", rec.FinalScore, "passed", rec.Passed, "teacher", actor.Name)
	return rec, nil
}

// ListPending 教师分页查询待复核记录。
func (s *GradeReviewService) ListPending(ctx context.Context, page, pageSize int64) ([]*model.ExamRecord, int64, error) {
	list, total, err := s.repo.ListPendingReviews(ctx, page, pageSize)
	if err != nil {
		return nil, 0, fmt.Errorf("grade review service list pending: %w", err)
	}
	return list, total, nil
}

// snapshotSubjectiveScores 固化当前主观题题分快照（修改前/修改后各一份，随复核一次落盘）。
func snapshotSubjectiveScores(questions []model.AttemptQuestion) []model.ScoreSnapshot {
	snaps := make([]model.ScoreSnapshot, 0)
	for i := range questions {
		q := &questions[i]
		if constants.IsObjectiveQuestion(q.Type) {
			continue
		}
		snaps = append(snaps, model.ScoreSnapshot{
			QuestionID:      q.QuestionID,
			Type:            q.Type,
			SubjectiveScore: q.SubjectiveScore,
			GotScore:        q.GotScore,
			Result:          q.Result,
			Comment:         q.Comment,
		})
	}
	return snaps
}
