package service

import (
	"context"
	"io"
	"log/slog"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"

	"github.com/onlineexam/onlineexam/internal/constants"
	"github.com/onlineexam/onlineexam/internal/dto"
	"github.com/onlineexam/onlineexam/internal/util"
)

// newReviewFixture 构造「已批改发布」的试卷+记录，返回服务、师生 ID 与题目 ID。
func newReviewFixture(t *testing.T) (*ExamRecordService, *GradeReviewService, primitive.ObjectID, primitive.ObjectID, string, string) {
	t.Helper()
	svc := newTestRecordSvc()
	reviewSvc := NewGradeReviewService(svc.repo, slog.New(slog.NewTextHandler(io.Discard, nil)))
	teacher := primitive.NewObjectID()
	student := primitive.NewObjectID()

	singleReq := &dto.CreateQuestionRequest{
		Type: "single", Subject: "数学", KnowledgePoints: []string{"代数"},
		Difficulty: "easy", Content: "1+1=?", Answer: "B", Score: 5,
		Options: []dto.OptionInput{{Key: "A", Text: "1"}, {Key: "B", Text: "2"}},
	}
	q1, _ := svc.exam.question.Create(context.Background(), singleReq, teacher)
	fillReq := &dto.CreateQuestionRequest{
		Type: "fill", Subject: "数学", KnowledgePoints: []string{"代数"},
		Difficulty: "medium", Content: "圆周率前两位是？", Answer: "3.14", Score: 5,
	}
	q2, _ := svc.exam.question.Create(context.Background(), fillReq, teacher)

	now := time.Now()
	exam, err := svc.exam.Create(context.Background(), &dto.CreateExamRequest{
		Title: "复核测试卷", Subject: "数学", DurationMin: 30, PassScore: 8,
		StartAt: now.Add(-time.Hour), EndAt: now.Add(time.Hour),
		Questions: []dto.ExamQuestionInput{{QuestionID: q1.ID.Hex()}, {QuestionID: q2.ID.Hex()}},
	}, teacher)
	if err != nil {
		t.Fatalf("create exam: %v", err)
	}
	_, _ = svc.exam.Publish(context.Background(), exam.ID, "t@example.com")

	rec, err := svc.StartExam(context.Background(), exam.ID, student, "李同学")
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	_, err = svc.Submit(context.Background(), rec.ID, []dto.AnswerInput{{QuestionID: q1.ID.Hex(), Answer: "B"}}, 0, nil, false)
	if err != nil {
		t.Fatalf("submit: %v", err)
	}
	graded, err := svc.Grade(context.Background(), rec.ID, []dto.GradeItem{{QuestionID: q2.ID.Hex(), Score: 2, Comment: "部分正确"}}, "t@example.com")
	if err != nil {
		t.Fatalf("grade: %v", err)
	}
	return svc, reviewSvc, teacher, student, graded.ID.Hex(), q2.ID.Hex()
}

func reviewActor(id primitive.ObjectID, name string) ReviewActor {
	return ReviewActor{UserID: id, Name: name, IP: "127.0.0.1", RequestID: "req-test"}
}

// TestReviewHappyPath 申请 → 锁定 → 教师改主观题 → 重算总分与及格 → 审计快照完整。
func TestReviewHappyPath(t *testing.T) {
	svc, reviewSvc, teacher, student, recordID, fillQID := newReviewFixture(t)
	rid, _ := primitive.ObjectIDFromHex(recordID)

	rec, err := reviewSvc.RequestReview(context.Background(), rid, "填空题给分偏低，申请复核", reviewActor(student, "李同学"))
	if err != nil {
		t.Fatalf("RequestReview() error = %v", err)
	}
	if rec.Review == nil || rec.Review.Status != constants.ReviewStatusPending {
		t.Fatalf("review status = %v, want pending", rec.Review)
	}
	if rec.Review.Audit.BeforeFinalScore != 7 {
		t.Fatalf("before final = %v, want 7", rec.Review.Audit.BeforeFinalScore)
	}
	if rec.Review.Audit.BeforePassed {
		t.Fatal("7/8 不应及格")
	}
	if len(rec.Review.BeforeScores) != 1 || rec.Review.BeforeScores[0].SubjectiveScore != 2 {
		t.Fatalf("before scores snapshot wrong: %+v", rec.Review.BeforeScores)
	}

	// 待复核期间普通批改被拒绝（成绩锁定）。
	if _, err := svc.Grade(context.Background(), rid, []dto.GradeItem{{QuestionID: fillQID, Score: 1}}, "t@example.com"); err == nil {
		t.Fatal("待复核期间批改应失败")
	} else if ae, ok := err.(*util.AppError); !ok || ae.Code != constants.CodeReviewLocked {
		t.Fatalf("锁定错误码 = %v", err)
	}

	// 教师复核：主观题 2 → 5，总分 7 → 10，及格结果 false → true。
	done, err := reviewSvc.CompleteReview(context.Background(), rid, dto.CompleteReviewRequest{
		Opinion: "经复核参考答案标准放宽，补至满分",
		Grades:  []dto.ReviewGradeItem{{QuestionID: fillQID, Score: 5, Comment: "复核补分"}},
	}, reviewActor(teacher, "王老师"))
	if err != nil {
		t.Fatalf("CompleteReview() error = %v", err)
	}
	if done.Review.Status != constants.ReviewStatusAdjusted {
		t.Fatalf("status = %s, want adjusted", done.Review.Status)
	}
	if done.FinalScore != 10 {
		t.Fatalf("final = %v, want 10", done.FinalScore)
	}
	if !done.Passed {
		t.Fatal("10/8 应及格")
	}
	if done.SubjectiveScore != 5 {
		t.Fatalf("subjective = %v, want 5", done.SubjectiveScore)
	}
	if done.Review.Audit.AfterFinalScore != 10 || !done.Review.Audit.AfterPassed {
		t.Fatalf("after audit snapshot wrong: %+v", done.Review.Audit)
	}
	if len(done.Review.Audit.ScoreChanges) != 1 ||
		done.Review.Audit.ScoreChanges[0].Before != 2 || done.Review.Audit.ScoreChanges[0].After != 5 {
		t.Fatalf("score changes wrong: %+v", done.Review.Audit.ScoreChanges)
	}
	if done.Review.Audit.HandledByName != "王老师" || done.Review.Audit.HandledAt == nil {
		t.Fatal("处理人审计缺失")
	}
}

// TestReviewRejectedWhenScoreUnchanged 分值无变化 → rejected，维持原分。
func TestReviewRejectedWhenScoreUnchanged(t *testing.T) {
	_, reviewSvc, teacher, student, recordID, fillQID := newReviewFixture(t)
	rid, _ := primitive.ObjectIDFromHex(recordID)
	if _, err := reviewSvc.RequestReview(context.Background(), rid, "申请复核", reviewActor(student, "李同学")); err != nil {
		t.Fatalf("request: %v", err)
	}
	done, err := reviewSvc.CompleteReview(context.Background(), rid, dto.CompleteReviewRequest{
		Opinion: "经核查评分无误",
		Grades:  []dto.ReviewGradeItem{{QuestionID: fillQID, Score: 2}},
	}, reviewActor(teacher, "王老师"))
	if err != nil {
		t.Fatalf("complete: %v", err)
	}
	if done.Review.Status != constants.ReviewStatusRejected {
		t.Fatalf("status = %s, want rejected", done.Review.Status)
	}
	if done.FinalScore != 7 || done.Passed {
		t.Fatalf("分数应维持 7/不及格，got %v/%v", done.FinalScore, done.Passed)
	}
}

// TestReviewBusinessRules 申请侧规则：窗口外、重复、非本人。
func TestReviewBusinessRules(t *testing.T) {
	_, reviewSvc, _, student, recordID, _ := newReviewFixture(t)
	rid, _ := primitive.ObjectIDFromHex(recordID)

	// 非本人申请。
	if _, err := reviewSvc.RequestReview(context.Background(), rid, "非本人", reviewActor(primitive.NewObjectID(), "陌生人")); err == nil {
		t.Fatal("非本人申请应失败")
	} else if ae, ok := err.(*util.AppError); !ok || ae.Code != constants.CodeReviewNotOwner {
		t.Fatalf("错误码 = %v, want CodeReviewNotOwner", err)
	}

	// 本人正常申请一次。
	if _, err := reviewSvc.RequestReview(context.Background(), rid, "第一次申请", reviewActor(student, "李同学")); err != nil {
		t.Fatalf("first request: %v", err)
	}
	// 重复申请被拒（pending 中也算一次，闭环仅一次）。
	if _, err := reviewSvc.RequestReview(context.Background(), rid, "第二次申请", reviewActor(student, "李同学")); err == nil {
		t.Fatal("重复申请应失败")
	} else if ae, ok := err.(*util.AppError); !ok || ae.Code != constants.CodeReviewDuplicate {
		t.Fatalf("错误码 = %v, want CodeReviewDuplicate", err)
	}

	// 复核窗口外：另一条记录把 graded_at 回拨到 25 小时前。
	svc2, reviewSvc2, _, student2, recordID2, _ := newReviewFixture(t)
	rid2, _ := primitive.ObjectIDFromHex(recordID2)
	old, _ := svc2.repo.FindByID(context.Background(), rid2)
	expired := time.Now().Add(-25 * time.Hour)
	old.GradedAt = &expired
	old.UpdatedAt = time.Now()
	if err := svc2.repo.Update(context.Background(), old); err != nil {
		t.Fatalf("backdate: %v", err)
	}
	if _, err := reviewSvc2.RequestReview(context.Background(), rid2, "超窗口", reviewActor(student2, "李同学")); err == nil {
		t.Fatal("超 24h 窗口申请应失败")
	} else if ae, ok := err.(*util.AppError); !ok || ae.Code != constants.CodeReviewWindowClose {
		t.Fatalf("错误码 = %v, want CodeReviewWindowClose", err)
	}
}

// TestReviewObjectiveForbidden 复核时改客观题被拒绝，且文档保持原样。
func TestReviewObjectiveForbidden(t *testing.T) {
	svc, reviewSvc, teacher, student, recordID, _ := newReviewFixture(t)
	rid, _ := primitive.ObjectIDFromHex(recordID)
	if _, err := reviewSvc.RequestReview(context.Background(), rid, "申请", reviewActor(student, "李同学")); err != nil {
		t.Fatalf("request: %v", err)
	}
	// 找到客观题 id。
	rec, _ := svc.repo.FindByID(context.Background(), rid)
	var objectiveID string
	for _, q := range rec.Questions {
		if constants.IsObjectiveQuestion(q.Type) {
			objectiveID = q.QuestionID.Hex()
		}
	}
	_, err := reviewSvc.CompleteReview(context.Background(), rid, dto.CompleteReviewRequest{
		Opinion: "试图改客观题",
		Grades:  []dto.ReviewGradeItem{{QuestionID: objectiveID, Score: 0}},
	}, reviewActor(teacher, "王老师"))
	if err == nil {
		t.Fatal("改客观题应失败")
	}
	after, _ := svc.repo.FindByID(context.Background(), rid)
	if after.Review.Status != constants.ReviewStatusPending {
		t.Fatalf("失败后状态应仍为 pending，got %s", after.Review.Status)
	}
	if after.FinalScore != 7 {
		t.Fatalf("失败后成绩应保持 7，got %v", after.FinalScore)
	}
}

// TestReviewScoreOutOfRange 给分超出题目满分被拒。
func TestReviewScoreOutOfRange(t *testing.T) {
	_, reviewSvc, teacher, student, recordID, fillQID := newReviewFixture(t)
	rid, _ := primitive.ObjectIDFromHex(recordID)
	_, _ = reviewSvc.RequestReview(context.Background(), rid, "申请", reviewActor(student, "李同学"))
	if _, err := reviewSvc.CompleteReview(context.Background(), rid, dto.CompleteReviewRequest{
		Opinion: "超满分",
		Grades:  []dto.ReviewGradeItem{{QuestionID: fillQID, Score: 99}},
	}, reviewActor(teacher, "王老师")); err == nil {
		t.Fatal("超满分给分应失败")
	}
}

// TestReviewHandleNotFound 无 pending 复核时处理报错。
func TestReviewHandleNotFound(t *testing.T) {
	_, reviewSvc, teacher, _, recordID, fillQID := newReviewFixture(t)
	rid, _ := primitive.ObjectIDFromHex(recordID)
	if _, err := reviewSvc.CompleteReview(context.Background(), rid, dto.CompleteReviewRequest{
		Opinion: "无申请直接处理",
		Grades:  []dto.ReviewGradeItem{{QuestionID: fillQID, Score: 5}},
	}, reviewActor(teacher, "王老师")); err == nil {
		t.Fatal("无待处理复核应失败")
	}
}

// TestReviewPendingList 待复核列表只返回 pending 记录。
func TestReviewPendingList(t *testing.T) {
	_, reviewSvc, teacher, student, recordID, fillQID := newReviewFixture(t)
	rid, _ := primitive.ObjectIDFromHex(recordID)
	if _, err := reviewSvc.RequestReview(context.Background(), rid, "申请", reviewActor(student, "李同学")); err != nil {
		t.Fatalf("request: %v", err)
	}
	list, total, err := reviewSvc.ListPending(context.Background(), 1, 20)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if total != 1 || len(list) != 1 || list[0].ID != rid {
		t.Fatalf("pending list wrong: total=%d len=%d", total, len(list))
	}
	// 处理后不再出现在待复核列表。
	if _, err := reviewSvc.CompleteReview(context.Background(), rid, dto.CompleteReviewRequest{
		Opinion: "维持",
		Grades:  []dto.ReviewGradeItem{{QuestionID: fillQID, Score: 2}},
	}, reviewActor(teacher, "王老师")); err != nil {
		t.Fatalf("complete: %v", err)
	}
	_, total, _ = reviewSvc.ListPending(context.Background(), 1, 20)
	if total != 0 {
		t.Fatalf("处理后 pending 应为 0，got %d", total)
	}
}
