package model

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// 成绩复核状态枚举（GradeReviewStatus）：pending 待复核 / adjusted 已复核且分数有变化 /
// rejected 已复核但维持原分。
// 出现位置：model/grade_review.go、constants/enums.go、service/grade_review_service.go、
// handler/exam_record_handler.go、constants/error_codes.go、constants/log_templates.go、
// util/formatters.go、前端 src/constants/index.ts、src/utils/format.ts、src/app/records/review。
const (
	ReviewStatusPending  = "pending"  // 待复核：成绩锁定，教师不能重复批改
	ReviewStatusAdjusted = "adjusted" // 复核完成：主观题分值发生变化，总分/及格结果已重算
	ReviewStatusRejected = "rejected" // 复核完成：维持原分
)

// ScoreSnapshot 复核落盘时的题分快照（修改前/修改后各存一份）。
type ScoreSnapshot struct {
	QuestionID      primitive.ObjectID `bson:"question_id" json:"question_id"`
	Type            string             `bson:"type" json:"type"`
	SubjectiveScore float64            `bson:"subjective_score" json:"subjective_score"`
	GotScore        float64            `bson:"got_score" json:"got_score"`
	Result          string             `bson:"result" json:"result"`
	Comment         string             `bson:"comment,omitempty" json:"comment,omitempty"`
}

// ReviewScoreChange 单题分值变化明细（审计用，前端回读对比修改前后）。
type ReviewScoreChange struct {
	QuestionID string  `bson:"question_id" json:"question_id"`
	Type       string  `bson:"type" json:"type"`
	Before     float64 `bson:"before" json:"before"`
	After      float64 `bson:"after" json:"after"`
}

// ReviewAudit 复核闭环审计记录：与申请、结果同文档一次原子落盘。
// 记录申请与处理两个动作的操作人、时间、IP、请求追踪 ID，以及修改前后总分快照。
type ReviewAudit struct {
	RequestedBy      primitive.ObjectID `bson:"requested_by" json:"requested_by"`
	RequestedByName  string             `bson:"requested_by_name" json:"requested_by_name"`
	RequestedAt      time.Time          `bson:"requested_at" json:"requested_at"`
	RequestIP        string             `bson:"request_ip,omitempty" json:"request_ip,omitempty"`
	RequestRequestID string             `bson:"request_request_id,omitempty" json:"request_request_id,omitempty"`
	HandledBy        primitive.ObjectID `bson:"handled_by,omitempty" json:"handled_by,omitempty"`
	HandledByName    string             `bson:"handled_by_name,omitempty" json:"handled_by_name,omitempty"`
	HandledAt        *time.Time         `bson:"handled_at,omitempty" json:"handled_at,omitempty"`
	HandleIP         string             `bson:"handle_ip,omitempty" json:"handle_ip,omitempty"`
	HandleRequestID  string             `bson:"handle_request_id,omitempty" json:"handle_request_id,omitempty"`
	// 修改前后总分快照（任一主观题分值变化都重算总分）。
	BeforeObjectiveScore  float64             `bson:"before_objective_score" json:"before_objective_score"`
	BeforeSubjectiveScore float64             `bson:"before_subjective_score" json:"before_subjective_score"`
	BeforeFinalScore      float64             `bson:"before_final_score" json:"before_final_score"`
	BeforePassed          bool                `bson:"before_passed" json:"before_passed"`
	AfterObjectiveScore   float64             `bson:"after_objective_score" json:"after_objective_score"`
	AfterSubjectiveScore  float64             `bson:"after_subjective_score" json:"after_subjective_score"`
	AfterFinalScore       float64             `bson:"after_final_score" json:"after_final_score"`
	AfterPassed           bool                `bson:"after_passed" json:"after_passed"`
	ScoreChanges          []ReviewScoreChange `bson:"score_changes,omitempty" json:"score_changes,omitempty"`
}

// GradeReview 成绩复核申请与结果（内嵌于 ExamRecord，保证单文档原子落盘）。
type GradeReview struct {
	Status         string          `bson:"status" json:"status"`                                       // pending / adjusted / rejected
	Reason         string          `bson:"reason" json:"reason"`                                       // 学生申请理由（必填）
	TeacherOpinion string          `bson:"teacher_opinion,omitempty" json:"teacher_opinion,omitempty"` // 教师复核意见（必填）
	BeforeScores   []ScoreSnapshot `bson:"before_scores,omitempty" json:"before_scores,omitempty"`     // 申请时主观题分数快照
	AfterScores    []ScoreSnapshot `bson:"after_scores,omitempty" json:"after_scores,omitempty"`       // 复核后主观题分数快照
	Audit          ReviewAudit     `bson:"audit" json:"audit"`
	RequestedAt    time.Time       `bson:"requested_at" json:"requested_at"`
	HandledAt      *time.Time      `bson:"handled_at,omitempty" json:"handled_at,omitempty"`
	CreatedAt      time.Time       `bson:"created_at" json:"created_at"`
	UpdatedAt      time.Time       `bson:"updated_at" json:"updated_at"`
}
