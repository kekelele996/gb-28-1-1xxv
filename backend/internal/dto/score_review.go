package dto

import (
	"time"

	"github.com/onlineexam/onlineexam/internal/model"
)

// CreateReviewRequest 学生发起成绩复核请求。理由必填，且只能在成绩发布后 24h 内发起一次。
type CreateReviewRequest struct {
	Reason string `json:"reason" binding:"required,min=1,max=500"`
}

// ReviewGradeItem 教师复核时单题主观题改分（复核只能改主观题）。
type ReviewGradeItem struct {
	QuestionID string  `json:"question_id" binding:"required"`
	Score      float64 `json:"score" binding:"min=0,max=100"`
	Comment    string  `json:"comment" binding:"omitempty,max=500"`
}

// CompleteReviewRequest 教师复核裁定请求。
// Decision=adjusted 时 items 至少包含一项改分；Decision=rejected 维持原判时 items 可空。
// TeacherComment 复核意见必填。
type CompleteReviewRequest struct {
	Decision       string            `json:"decision" binding:"required,oneof=adjusted rejected"`
	TeacherComment string            `json:"teacher_comment" binding:"required,min=1,max=500"`
	Items          []ReviewGradeItem `json:"items" binding:"omitempty,dive"`
}

// ReviewChangeResponse 单题改分前后明细。
type ReviewChangeResponse struct {
	QuestionID    string  `json:"question_id"`
	Content       string  `json:"content"`
	QuestionType  string  `json:"question_type"`
	ScoreBefore   float64 `json:"score_before"`
	ScoreAfter    float64 `json:"score_after"`
	CommentBefore string  `json:"comment_before"`
	CommentAfter  string  `json:"comment_after"`
}

// ReviewAuditResponse 复核审计轨迹条目。
type ReviewAuditResponse struct {
	Action       string    `json:"action"`
	OperatorID   string    `json:"operator_id"`
	OperatorName string    `json:"operator_name"`
	Role         string    `json:"role"`
	Comment      string    `json:"comment"`
	CreatedAt    time.Time `json:"created_at"`
}

// ScoreReviewResponse 成绩复核记录响应（申请、复核、回读三态共用）。
type ScoreReviewResponse struct {
	RecordID         string                 `json:"record_id"`
	ExamID           string                 `json:"exam_id"`
	StudentID        string                 `json:"student_id"`
	ExamTitle        string                 `json:"exam_title"`
	StudentName      string                 `json:"student_name"`
	Reason           string                 `json:"reason"`
	Status           string                 `json:"status"`
	FinalScoreBefore float64                `json:"final_score_before"`
	FinalScoreAfter  float64                `json:"final_score_after"`
	PassedBefore     *bool                  `json:"passed_before"`
	PassedAfter      *bool                  `json:"passed_after"`
	PassScore        float64                `json:"pass_score"`
	Changes          []ReviewChangeResponse `json:"changes"`
	TeacherID        string                 `json:"teacher_id"`
	TeacherName      string                 `json:"teacher_name"`
	TeacherComment   string                 `json:"teacher_comment"`
	AppliedAt        *time.Time             `json:"applied_at"`
	DecidedAt        *time.Time             `json:"decided_at"`
	ExpiresAt        time.Time              `json:"expires_at"`
	AuditTrail       []ReviewAuditResponse  `json:"audit_trail"`
}

// ToScoreReviewResponse 模型转响应（nil 安全）。
func ToScoreReviewResponse(r *model.ScoreReview) *ScoreReviewResponse {
	if r == nil {
		return nil
	}
	resp := &ScoreReviewResponse{
		RecordID:         r.RecordID.Hex(),
		ExamID:           r.ExamID.Hex(),
		StudentID:        r.StudentID.Hex(),
		ExamTitle:        r.ExamTitle,
		StudentName:      r.StudentName,
		Reason:           r.Reason,
		Status:           r.Status,
		FinalScoreBefore: r.FinalScoreBefore,
		FinalScoreAfter:  r.FinalScoreAfter,
		PassedBefore:     r.PassedBefore,
		PassedAfter:      r.PassedAfter,
		PassScore:        r.PassScore,
		TeacherName:      r.TeacherName,
		TeacherComment:   r.TeacherComment,
		AppliedAt:        r.AppliedAt,
		DecidedAt:        r.DecidedAt,
		ExpiresAt:        r.ExpiresAt,
		Changes:          make([]ReviewChangeResponse, 0, len(r.Changes)),
		AuditTrail:       make([]ReviewAuditResponse, 0, len(r.AuditTrail)),
	}
	if r.TeacherID != nil {
		resp.TeacherID = r.TeacherID.Hex()
	}
	for _, c := range r.Changes {
		resp.Changes = append(resp.Changes, ReviewChangeResponse{
			QuestionID:    c.QuestionID.Hex(),
			Content:       c.Content,
			QuestionType:  c.QuestionType,
			ScoreBefore:   c.ScoreBefore,
			ScoreAfter:    c.ScoreAfter,
			CommentBefore: c.CommentBefore,
			CommentAfter:  c.CommentAfter,
		})
	}
	for _, a := range r.AuditTrail {
		resp.AuditTrail = append(resp.AuditTrail, ReviewAuditResponse{
			Action:       a.Action,
			OperatorID:   a.OperatorID.Hex(),
			OperatorName: a.OperatorName,
			Role:         a.Role,
			Comment:      a.Comment,
			CreatedAt:    a.CreatedAt,
		})
	}
	return resp
}
