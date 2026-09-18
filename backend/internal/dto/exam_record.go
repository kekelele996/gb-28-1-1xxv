package dto

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"

	"github.com/onlineexam/onlineexam/internal/constants"
	"github.com/onlineexam/onlineexam/internal/model"
)

// StartExamRequest 开始考试请求（无业务字段，预留防作弊配置）。
type StartExamRequest struct {
	ExamID string `json:"exam_id" binding:"required"`
}

// AnswerInput 单题作答输入。
type AnswerInput struct {
	QuestionID string `json:"question_id" binding:"required"`
	Answer     string `json:"answer" binding:"max=2000"`
}

// SubmitRecordRequest 提交答卷请求。
type SubmitRecordRequest struct {
	Answers     []AnswerInput     `json:"answers" binding:"required,min=1"`
	CheatCount  int               `json:"cheat_count" binding:"omitempty,min=0,max=1000"`
	CheatEvents []CheatEventInput `json:"cheat_events"`
}

// CheatEventInput 防作弊事件输入。
type CheatEventInput struct {
	Type   string `json:"type" binding:"omitempty,oneof=switch_tab copy_paste blur"`
	Detail string `json:"detail" binding:"omitempty,max=500"`
}

// GradeRequest 主观题批改请求。
type GradeRequest struct {
	Grades []GradeItem `json:"grades" binding:"required,min=1"`
}

// GradeItem 单题批改。
type GradeItem struct {
	QuestionID string  `json:"question_id" binding:"required"`
	Score      float64 `json:"score" binding:"min=0,max=100"`
	Comment    string  `json:"comment" binding:"omitempty,max=500"`
}

// RequestReviewRequest 学生发起成绩复核请求（理由必填，24h 窗口内仅一次）。
type RequestReviewRequest struct {
	Reason string `json:"reason" binding:"required,min=1,max=500"`
}

// ReviewGradeItem 教师复核单题给分（仅主观题）。
type ReviewGradeItem struct {
	QuestionID string  `json:"question_id" binding:"required"`
	Score      float64 `json:"score" binding:"min=0,max=100"`
	Comment    string  `json:"comment" binding:"omitempty,max=500"`
}

// CompleteReviewRequest 教师处理复核请求：复核意见必填，给分可选（不给分表示维持原分）。
// result 可选 adjusted/rejected，不传则按分值是否变化自动判定。
type CompleteReviewRequest struct {
	Opinion string            `json:"teacher_opinion" binding:"required,min=1,max=500"`
	Result  string            `json:"result" binding:"omitempty,oneof=adjusted rejected"`
	Grades  []ReviewGradeItem `json:"grades" binding:"omitempty,dive"`
}

// RecordQuery 考试记录查询参数。
type RecordQuery struct {
	ExamID    string `form:"exam_id"`
	Status    string `form:"status"`
	StudentID string `form:"student_id"`
	Page      int64  `form:"page"`
	PageSize  int64  `form:"page_size"`
}

// ExamReportItem 成绩分析单题正确率。
type ExamReportItem struct {
	QuestionID   string  `json:"question_id"`
	Content      string  `json:"content"`
	Type         string  `json:"type"`
	AnswerCount  int     `json:"answer_count"`
	CorrectCount int     `json:"correct_count"`
	Accuracy     float64 `json:"accuracy"`
}

// ExamReport 成绩分析报告。
type ExamReport struct {
	ExamID          string           `json:"exam_id"`
	ExamTitle       string           `json:"exam_title"`
	TotalStudents   int              `json:"total_students"`
	AverageScore    float64          `json:"average_score"`
	MaxScore        float64          `json:"max_score"`
	MinScore        float64          `json:"min_score"`
	PassRate        float64          `json:"pass_rate"`
	ScoreBands      map[string]int   `json:"score_bands"` // 分数段直方图
	QuestionReports []ExamReportItem `json:"question_reports"`
}

// ScoreSnapshotResponse 复核题分快照（修改前/修改后）。
type ScoreSnapshotResponse struct {
	QuestionID      string  `json:"question_id"`
	Type            string  `json:"type"`
	SubjectiveScore float64 `json:"subjective_score"`
	GotScore        float64 `json:"got_score"`
	Result          string  `json:"result"`
	Comment         string  `json:"comment,omitempty"`
}

// ReviewScoreChangeResponse 复核单题分值变化（审计回读）。
type ReviewScoreChangeResponse struct {
	QuestionID string  `json:"question_id"`
	Type       string  `json:"type"`
	Before     float64 `json:"before"`
	After      float64 `json:"after"`
}

// ReviewAuditResponse 复核审计记录（申请/处理动作、修改前后总分快照）。
type ReviewAuditResponse struct {
	RequestedBy           string                      `json:"requested_by"`
	RequestedByName       string                      `json:"requested_by_name"`
	RequestedAt           time.Time                   `json:"requested_at"`
	RequestIP             string                      `json:"request_ip,omitempty"`
	RequestRequestID      string                      `json:"request_request_id,omitempty"`
	HandledBy             string                      `json:"handled_by,omitempty"`
	HandledByName         string                      `json:"handled_by_name,omitempty"`
	HandledAt             *time.Time                  `json:"handled_at,omitempty"`
	HandleIP              string                      `json:"handle_ip,omitempty"`
	HandleRequestID       string                      `json:"handle_request_id,omitempty"`
	BeforeObjectiveScore  float64                     `json:"before_objective_score"`
	BeforeSubjectiveScore float64                     `json:"before_subjective_score"`
	BeforeFinalScore      float64                     `json:"before_final_score"`
	BeforePassed          bool                        `json:"before_passed"`
	AfterObjectiveScore   float64                     `json:"after_objective_score"`
	AfterSubjectiveScore  float64                     `json:"after_subjective_score"`
	AfterFinalScore       float64                     `json:"after_final_score"`
	AfterPassed           bool                        `json:"after_passed"`
	ScoreChanges          []ReviewScoreChangeResponse `json:"score_changes,omitempty"`
}

// GradeReviewResponse 成绩复核申请与结果（页面按角色回读申请、复核、审计状态）。
type GradeReviewResponse struct {
	Status         string                  `json:"status"`
	Reason         string                  `json:"reason"`
	TeacherOpinion string                  `json:"teacher_opinion,omitempty"`
	BeforeScores   []ScoreSnapshotResponse `json:"before_scores,omitempty"`
	AfterScores    []ScoreSnapshotResponse `json:"after_scores,omitempty"`
	Audit          ReviewAuditResponse     `json:"audit"`
	RequestedAt    time.Time               `json:"requested_at"`
	HandledAt      *time.Time              `json:"handled_at,omitempty"`
}

// RecordResponse 考试记录响应。
type RecordResponse struct {
	ID              string                  `json:"id"`
	ExamID          string                  `json:"exam_id"`
	ExamTitle       string                  `json:"exam_title"`
	StudentID       string                  `json:"student_id"`
	StudentName     string                  `json:"student_name"`
	Status          string                  `json:"status"`
	StartedAt       time.Time               `json:"started_at"`
	SubmittedAt     *time.Time              `json:"submitted_at"`
	GradedAt        *time.Time              `json:"graded_at"`
	ObjectiveScore  float64                 `json:"objective_score"`
	SubjectiveScore float64                 `json:"subjective_score"`
	FinalScore      float64                 `json:"final_score"`
	PassScore       float64                 `json:"pass_score"`
	Passed          bool                    `json:"passed"`
	CheatCount      int                     `json:"cheat_count"`
	AutoSubmitted   bool                    `json:"auto_submitted"`
	Questions       []model.AttemptQuestion `json:"questions"`
	Review          *GradeReviewResponse    `json:"review,omitempty"`
	ReviewDeadline  *time.Time              `json:"review_deadline,omitempty"` // 成绩发布 +24h（前端控制申请入口显隐）
	CreatedAt       time.Time               `json:"created_at"`
}

// ToScoreSnapshotResponse 题分快照模型转响应。
func ToScoreSnapshotResponse(s model.ScoreSnapshot) ScoreSnapshotResponse {
	return ScoreSnapshotResponse{
		QuestionID:      s.QuestionID.Hex(),
		Type:            s.Type,
		SubjectiveScore: s.SubjectiveScore,
		GotScore:        s.GotScore,
		Result:          s.Result,
		Comment:         s.Comment,
	}
}

// ToGradeReviewResponse 复核模型转响应（含审计与修改前后分数）。
func ToGradeReviewResponse(r *model.GradeReview) GradeReviewResponse {
	resp := GradeReviewResponse{
		Status:         r.Status,
		Reason:         r.Reason,
		TeacherOpinion: r.TeacherOpinion,
		RequestedAt:    r.RequestedAt,
		HandledAt:      r.HandledAt,
		Audit: ReviewAuditResponse{
			RequestedBy:           r.Audit.RequestedBy.Hex(),
			RequestedByName:       r.Audit.RequestedByName,
			RequestedAt:           r.Audit.RequestedAt,
			RequestIP:             r.Audit.RequestIP,
			RequestRequestID:      r.Audit.RequestRequestID,
			HandledBy:             oidToHex(r.Audit.HandledBy),
			HandledByName:         r.Audit.HandledByName,
			HandledAt:             r.Audit.HandledAt,
			HandleIP:              r.Audit.HandleIP,
			HandleRequestID:       r.Audit.HandleRequestID,
			BeforeObjectiveScore:  r.Audit.BeforeObjectiveScore,
			BeforeSubjectiveScore: r.Audit.BeforeSubjectiveScore,
			BeforeFinalScore:      r.Audit.BeforeFinalScore,
			BeforePassed:          r.Audit.BeforePassed,
			AfterObjectiveScore:   r.Audit.AfterObjectiveScore,
			AfterSubjectiveScore:  r.Audit.AfterSubjectiveScore,
			AfterFinalScore:       r.Audit.AfterFinalScore,
			AfterPassed:           r.Audit.AfterPassed,
		},
	}
	for _, s := range r.BeforeScores {
		resp.BeforeScores = append(resp.BeforeScores, ToScoreSnapshotResponse(s))
	}
	for _, s := range r.AfterScores {
		resp.AfterScores = append(resp.AfterScores, ToScoreSnapshotResponse(s))
	}
	for _, c := range r.Audit.ScoreChanges {
		resp.Audit.ScoreChanges = append(resp.Audit.ScoreChanges, ReviewScoreChangeResponse{
			QuestionID: c.QuestionID,
			Type:       c.Type,
			Before:     c.Before,
			After:      c.After,
		})
	}
	return resp
}

// oidToHex ObjectID 转 hex，零值返回空串。
func oidToHex(id primitive.ObjectID) string {
	if id.IsZero() {
		return ""
	}
	return id.Hex()
}

// ToRecordResponse 模型转响应。
func ToRecordResponse(r *model.ExamRecord) RecordResponse {
	resp := RecordResponse{
		ID:              r.ID.Hex(),
		ExamID:          r.ExamID.Hex(),
		ExamTitle:       r.ExamTitle,
		StudentID:       r.StudentID.Hex(),
		StudentName:     r.StudentName,
		Status:          r.Status,
		StartedAt:       r.StartedAt,
		SubmittedAt:     r.SubmittedAt,
		GradedAt:        r.GradedAt,
		ObjectiveScore:  r.ObjectiveScore,
		SubjectiveScore: r.SubjectiveScore,
		FinalScore:      r.FinalScore,
		PassScore:       r.PassScore,
		Passed:          r.Passed,
		CheatCount:      r.CheatCount,
		AutoSubmitted:   r.AutoSubmitted,
		Questions:       r.Questions,
		CreatedAt:       r.CreatedAt,
	}
	if r.Review != nil {
		rv := ToGradeReviewResponse(r.Review)
		resp.Review = &rv
	}
	if r.GradedAt != nil {
		deadline := r.GradedAt.Add(constants.ReviewWindow)
		resp.ReviewDeadline = &deadline
	}
	return resp
}
