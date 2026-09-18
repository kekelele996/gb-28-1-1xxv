package model

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// 成绩复核（ScoreReview）相关模型，单独成文件（每实体一个 model 文件）。
// 复核记录内嵌在 ExamRecord 同一文档（exam_records.review）：MongoDB 单文档写天然原子，
// 申请与复核各只需一次条件写（配合 version 乐观锁），写入失败或并发冲突时成绩与复核记录保持原样，
// 满足“复核意见、修改前后分数和审计记录一次落盘；失败时记录、成绩和审计保持原样”。

// ScoreReviewChange 复核改分明细：记录单题（主观题）修改前后的分值与评语。
type ScoreReviewChange struct {
	QuestionID    primitive.ObjectID `bson:"question_id" json:"question_id"`
	Content       string             `bson:"content" json:"content"`             // 题干快照
	QuestionType  string             `bson:"question_type" json:"question_type"` // 题型快照（fill/short）
	ScoreBefore   float64            `bson:"score_before" json:"score_before"`
	ScoreAfter    float64            `bson:"score_after" json:"score_after"`
	CommentBefore string             `bson:"comment_before" json:"comment_before"`
	CommentAfter  string             `bson:"comment_after" json:"comment_after"`
}

// ReviewAuditEntry 成绩复核审计轨迹（申请/复核各一条，与成绩、复核记录同文档一次落盘）。
type ReviewAuditEntry struct {
	Action       string             `bson:"action" json:"action"` // review_apply / review_decide
	OperatorID   primitive.ObjectID `bson:"operator_id" json:"operator_id"`
	OperatorName string             `bson:"operator_name" json:"operator_name"`
	Role         string             `bson:"role" json:"role"`
	Comment      string             `bson:"comment" json:"comment"` // 申请理由 / 复核意见
	CreatedAt    time.Time          `bson:"created_at" json:"created_at"`
}

// ScoreReview 成绩复核记录。
// 状态枚举：pending（待复核）/ adjusted（已调整）/ rejected（维持原判）。
type ScoreReview struct {
	RecordID         primitive.ObjectID  `bson:"record_id" json:"record_id"`
	ExamID           primitive.ObjectID  `bson:"exam_id" json:"exam_id"`
	StudentID        primitive.ObjectID  `bson:"student_id" json:"student_id"`
	ExamTitle        string              `bson:"exam_title" json:"exam_title"`
	StudentName      string              `bson:"student_name" json:"student_name"`
	Reason           string              `bson:"reason" json:"reason"` // 申请理由（必填）
	Status           string              `bson:"status" json:"status"` // pending/adjusted/rejected
	FinalScoreBefore float64             `bson:"final_score_before" json:"final_score_before"`
	FinalScoreAfter  float64             `bson:"final_score_after" json:"final_score_after"`
	PassedBefore     *bool               `bson:"passed_before,omitempty" json:"passed_before"`
	PassedAfter      *bool               `bson:"passed_after,omitempty" json:"passed_after"`
	PassScore        float64             `bson:"pass_score" json:"pass_score"` // 原及格线快照
	Changes          []ScoreReviewChange `bson:"changes" json:"changes"`
	TeacherID        *primitive.ObjectID `bson:"teacher_id,omitempty" json:"teacher_id"`
	TeacherName      string              `bson:"teacher_name,omitempty" json:"teacher_name"`
	TeacherComment   string              `bson:"teacher_comment,omitempty" json:"teacher_comment"` // 复核意见（必填）
	AppliedAt        *time.Time          `bson:"applied_at" json:"applied_at"`
	DecidedAt        *time.Time          `bson:"decided_at,omitempty" json:"decided_at"`
	ExpiresAt        time.Time           `bson:"expires_at" json:"expires_at"` // 成绩发布 +24h
	AuditTrail       []ReviewAuditEntry  `bson:"audit_trail" json:"audit_trail"`
}
