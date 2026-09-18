package service

import (
	"context"
	"io"
	"log/slog"
	"sync"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"

	"github.com/onlineexam/onlineexam/internal/constants"
	"github.com/onlineexam/onlineexam/internal/dto"
	"github.com/onlineexam/onlineexam/internal/model"
)

// fakeReviewRepo 内存版成绩复核仓储（复用 fakeRecordRepo 的数据）。
type fakeReviewRepo struct {
	records *fakeRecordRepo
}

// fakeAuditSink 记录系统级复核审计事件。
type fakeAuditSink struct {
	mu     sync.Mutex
	events []ReviewAuditEvent
}

func (f *fakeAuditSink) WriteReviewAudit(_ context.Context, e ReviewAuditEvent) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.events = append(f.events, e)
}

func (f *fakeAuditSink) len() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.events)
}

func (f *fakeReviewRepo) ListByStatus(_ context.Context, statuses []string, _, _ int64) ([]*model.ExamRecord, int64, error) {
	f.records.mu.Lock()
	defer f.records.mu.Unlock()
	var out []*model.ExamRecord
	for _, r := range f.records.records {
		if r.Review != nil && containsStr(statuses, r.Review.Status) {
			out = append(out, r)
		}
	}
	return out, int64(len(out)), nil
}

func (f *fakeReviewRepo) ListByStudent(_ context.Context, studentID primitive.ObjectID, statuses []string, _, _ int64) ([]*model.ExamRecord, int64, error) {
	f.records.mu.Lock()
	defer f.records.mu.Unlock()
	var out []*model.ExamRecord
	for _, r := range f.records.records {
		if r.Review == nil || r.Review.StudentID != studentID {
			continue
		}
		if len(statuses) == 0 || containsStr(statuses, r.Review.Status) {
			out = append(out, r)
		}
	}
	return out, int64(len(out)), nil
}

func containsStr(list []string, v string) bool {
	for _, s := range list {
		if s == v {
			return true
		}
	}
	return false
}

// reviewFixture 构造“已发布试卷 + 已提交并已批改”的答卷，返回服务与关键 ID。
type reviewFixture struct {
	svc      *ScoreReviewService
	recSvc   *ExamRecordService
	recordID primitive.ObjectID
	student  primitive.ObjectID
	teacher  primitive.ObjectID
	fillQID  primitive.ObjectID
	recRepo  *fakeRecordRepo
	sink     *fakeAuditSink
}

func setupReviewFixture(t *testing.T, passScore float64) reviewFixture {
	t.Helper()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	questionRepo := newFakeQuestionRepo()
	questionSvc := NewQuestionService(questionRepo, logger)
	examRepo := newFakeExamRepo()
	examSvc := NewExamService(examRepo, questionSvc, logger)
	recordRepo := newFakeRecordRepo()
	reviewRepo := &fakeReviewRepo{records: recordRepo}
	sink := &fakeAuditSink{}
	recSvc := NewExamRecordService(recordRepo, examSvc, logger)
	svc := NewScoreReviewService(recordRepo, reviewRepo, sink, logger)

	teacher := primitive.NewObjectID()
	student := primitive.NewObjectID()
	now := time.Now()

	singleReq := &dto.CreateQuestionRequest{
		Type: "single", Subject: "数学", KnowledgePoints: []string{"代数"},
		Difficulty: "easy", Content: "1+1=?", Answer: "B", Score: 5,
		Options: []dto.OptionInput{{Key: "A", Text: "1"}, {Key: "B", Text: "2"}},
	}
	q1, _ := questionSvc.Create(context.Background(), singleReq, teacher)
	fillReq := &dto.CreateQuestionRequest{
		Type: "fill", Subject: "数学", KnowledgePoints: []string{"代数"},
		Difficulty: "medium", Content: "圆周率前两位是？", Answer: "3.14", Score: 5,
	}
	q2, _ := questionSvc.Create(context.Background(), fillReq, teacher)

	exam, err := examSvc.Create(context.Background(), &dto.CreateExamRequest{
		Title: "复核测试卷", Subject: "数学", DurationMin: 30, PassScore: passScore,
		StartAt: now.Add(-time.Hour), EndAt: now.Add(time.Hour),
		Questions: []dto.ExamQuestionInput{{QuestionID: q1.ID.Hex()}, {QuestionID: q2.ID.Hex()}},
	}, teacher)
	if err != nil {
		t.Fatalf("create exam: %v", err)
	}
	if _, err := examSvc.Publish(context.Background(), exam.ID, "t@example.com"); err != nil {
		t.Fatalf("publish: %v", err)
	}
	rec, err := recSvc.StartExam(context.Background(), exam.ID, student, "李同学")
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	if _, err := recSvc.Submit(context.Background(), rec.ID,
		[]dto.AnswerInput{{QuestionID: q1.ID.Hex(), Answer: "B"}}, 0, nil, false); err != nil {
		t.Fatalf("submit: %v", err)
	}
	if _, err := recSvc.Grade(context.Background(), rec.ID,
		[]dto.GradeItem{{QuestionID: q2.ID.Hex(), Score: 0, Comment: "答错"}}, "t@example.com"); err != nil {
		t.Fatalf("grade: %v", err)
	}
	return reviewFixture{
		svc: svc, recSvc: recSvc, recordID: rec.ID, student: student,
		teacher: teacher, fillQID: q2.ID, recRepo: recordRepo, sink: sink,
	}
}

func TestReviewApplyAndAdjustFlow(t *testing.T) {
	fx := setupReviewFixture(t, 6)
	ctx := context.Background()

	rec, err := fx.svc.GetForViewer(ctx, fx.recordID, fx.student, constants.RoleStudent)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if rec.Status != constants.RecordStatusGraded || rec.GradedAt == nil {
		t.Fatal("成绩应已发布")
	}
	if rec.FinalScore != 5 || rec.Passed {
		t.Fatalf("初始总分=%v 及格=%v，期望 5/不及格", rec.FinalScore, rec.Passed)
	}

	// 学生在 24h 内发起复核
	applied, err := fx.svc.Apply(ctx, fx.recordID, fx.student, "李同学", "填空题作答应为 3.14，教师漏判")
	if err != nil {
		t.Fatalf("Apply() error = %v", err)
	}
	if applied.Review == nil || applied.Review.Status != constants.ReviewStatusPending {
		t.Fatal("复核应处于待复核")
	}
	if applied.Version != 3 {
		t.Fatalf("version = %d, want 3（提交、批改、申请各 +1）", applied.Version)
	}
	// 系统级审计：申请事件 1 条
	if fx.sink.len() != 1 {
		t.Fatalf("系统审计事件=%d，期望 1", fx.sink.len())
	}

	// 待复核期间成绩锁定：教师不能重复批改
	if _, err := fx.recSvc.Grade(ctx, fx.recordID,
		[]dto.GradeItem{{QuestionID: fx.fillQID.Hex(), Score: 2, Comment: "x"}}, "t@example.com"); err == nil {
		t.Fatal("待复核期间教师批改应被锁定拒绝")
	}
	// 锁定期间成绩保持原样
	locked, _ := fx.recSvc.GetByID(ctx, fx.recordID)
	if locked.FinalScore != 5 || locked.SubjectiveScore != 0 {
		t.Fatalf("锁定期间成绩被改动: final=%v subjective=%v", locked.FinalScore, locked.SubjectiveScore)
	}

	// 教师复核：把填空 0 分改为 5 分（主观题），重算总分 10，按原及格线 6 变为及格
	decided, err := fx.svc.Complete(ctx, fx.recordID, fx.teacher, "王老师", dto.CompleteReviewRequest{
		Decision:       constants.ReviewStatusAdjusted,
		TeacherComment: "经核实予以更正",
		Items: []dto.ReviewGradeItem{
			{QuestionID: fx.fillQID.Hex(), Score: 5, Comment: "答案正确"},
		},
	})
	if err != nil {
		t.Fatalf("Complete() error = %v", err)
	}
	if decided.Review.Status != constants.ReviewStatusAdjusted {
		t.Fatalf("status=%s", decided.Review.Status)
	}
	if decided.FinalScore != 10 {
		t.Fatalf("复核后总分=%v，期望 10", decided.FinalScore)
	}
	if !decided.Passed {
		t.Fatal("复核后应按原及格线 6 判定为及格")
	}
	if decided.SubjectiveScore != 5 {
		t.Fatalf("主观题总分=%v，期望 5", decided.SubjectiveScore)
	}
	ch := decided.Review.Changes[0]
	if ch.ScoreBefore != 0 || ch.ScoreAfter != 5 {
		t.Fatalf("改分快照错误: %+v", ch)
	}
	if *decided.Review.PassedBefore || decided.Review.PassedAfter == nil || !*decided.Review.PassedAfter {
		t.Fatal("及格状态前后快照错误")
	}
	// 审计轨迹：申请 + 裁定两条，一次落盘
	if len(decided.Review.AuditTrail) != 2 {
		t.Fatalf("审计条目=%d，期望 2", len(decided.Review.AuditTrail))
	}
	if decided.Review.AuditTrail[1].Action != constants.AuditActionReviewDecide ||
		decided.Review.AuditTrail[1].Comment != "经核实予以更正" {
		t.Fatal("复核意见未落盘")
	}
	// 系统级审计：申请 + 裁定共 2 条
	if fx.sink.len() != 2 {
		t.Fatalf("系统审计事件=%d，期望 2", fx.sink.len())
	}

	// 复核完成后教师可正常批改
	if _, err := fx.recSvc.Grade(ctx, fx.recordID,
		[]dto.GradeItem{{QuestionID: fx.fillQID.Hex(), Score: 4, Comment: "再核"}}, "t@example.com"); err != nil {
		t.Fatalf("复核完成后应允许批改: %v", err)
	}
}

func TestReviewRejectKeepsScore(t *testing.T) {
	fx := setupReviewFixture(t, 6)
	ctx := context.Background()
	if _, err := fx.svc.Apply(ctx, fx.recordID, fx.student, "李同学", "我觉得给低了"); err != nil {
		t.Fatalf("Apply() error = %v", err)
	}
	decided, err := fx.svc.Complete(ctx, fx.recordID, fx.teacher, "王老师", dto.CompleteReviewRequest{
		Decision: constants.ReviewStatusRejected, TeacherComment: "维持原判",
	})
	if err != nil {
		t.Fatalf("Complete() error = %v", err)
	}
	if decided.Review.Status != constants.ReviewStatusRejected {
		t.Fatalf("status=%s", decided.Review.Status)
	}
	if decided.FinalScore != 5 || decided.Passed {
		t.Fatal("维持原判不应改分")
	}
}

func TestReviewDuplicateSubmitRejected(t *testing.T) {
	fx := setupReviewFixture(t, 6)
	ctx := context.Background()
	if _, err := fx.svc.Apply(ctx, fx.recordID, fx.student, "李同学", "理由一"); err != nil {
		t.Fatalf("Apply() error = %v", err)
	}
	// 重复提交：拒绝，且第一次申请与成绩保持原样
	_, err := fx.svc.Apply(ctx, fx.recordID, fx.student, "李同学", "理由二")
	if err == nil {
		t.Fatal("重复提交复核应失败")
	}
	rec, _ := fx.recRepo.FindByID(ctx, fx.recordID)
	if rec.Review.Reason != "理由一" {
		t.Fatal("重复提交污染了原申请理由")
	}
	// 裁定后仍不能再次申请
	if _, err := fx.svc.Complete(ctx, fx.recordID, fx.teacher, "王老师", dto.CompleteReviewRequest{
		Decision: constants.ReviewStatusRejected, TeacherComment: "ok"}); err != nil {
		t.Fatalf("Complete() error = %v", err)
	}
	if _, err := fx.svc.Apply(ctx, fx.recordID, fx.student, "李同学", "理由三"); err == nil {
		t.Fatal("复核已存在（即使已裁定）也不能再次申请")
	}
}

func TestReviewWindowExpired(t *testing.T) {
	fx := setupReviewFixture(t, 6)
	ctx := context.Background()
	// 把成绩发布时间回拨到 25 小时前
	rec, _ := fx.recRepo.FindByID(ctx, fx.recordID)
	past := time.Now().Add(-25 * time.Hour)
	rec.GradedAt = &past
	if err := fx.recRepo.Update(ctx, rec); err != nil {
		t.Fatalf("update: %v", err)
	}
	if _, err := fx.svc.Apply(ctx, fx.recordID, fx.student, "李同学", "超期申请"); err == nil {
		t.Fatal("超过 24h 窗口应拒绝")
	}
	// 窗口边界：恰好 24h 内允许
	edge := time.Now().Add(-23 * time.Hour)
	rec.GradedAt = &edge
	_ = fx.recRepo.Update(ctx, rec)
	if _, err := fx.svc.Apply(ctx, fx.recordID, fx.student, "李同学", "窗口内申请"); err != nil {
		t.Fatalf("24h 内申请应成功: %v", err)
	}
}

func TestReviewNotGradedRejected(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	questionRepo := newFakeQuestionRepo()
	questionSvc := NewQuestionService(questionRepo, logger)
	examRepo := newFakeExamRepo()
	examSvc := NewExamService(examRepo, questionSvc, logger)
	recordRepo := newFakeRecordRepo()
	reviewRepo := &fakeReviewRepo{records: recordRepo}
	recSvc := NewExamRecordService(recordRepo, examSvc, logger)
	svc := NewScoreReviewService(recordRepo, reviewRepo, &fakeAuditSink{}, logger)

	teacher := primitive.NewObjectID()
	student := primitive.NewObjectID()
	now := time.Now()
	q1, _ := questionSvc.Create(context.Background(), &dto.CreateQuestionRequest{
		Type: "single", Subject: "数学", KnowledgePoints: []string{"代数"},
		Difficulty: "easy", Content: "1+1=?", Answer: "B", Score: 5,
		Options: []dto.OptionInput{{Key: "A", Text: "1"}, {Key: "B", Text: "2"}},
	}, teacher)
	exam, _ := examSvc.Create(context.Background(), &dto.CreateExamRequest{
		Title: "未发布成绩卷", Subject: "数学", DurationMin: 30, PassScore: 60,
		StartAt: now.Add(-time.Hour), EndAt: now.Add(time.Hour),
		Questions: []dto.ExamQuestionInput{{QuestionID: q1.ID.Hex()}},
	}, teacher)
	_, _ = examSvc.Publish(context.Background(), exam.ID, "t@example.com")
	rec, _ := recSvc.StartExam(context.Background(), exam.ID, student, "李同学")
	_, _ = recSvc.Submit(context.Background(), rec.ID, []dto.AnswerInput{{QuestionID: q1.ID.Hex(), Answer: "B"}}, 0, nil, false)

	if _, err := svc.Apply(context.Background(), rec.ID, student, "李同学", "还没批改呢"); err == nil {
		t.Fatal("未批改完成（成绩未发布）不能复核")
	}
}

func TestReviewObjectiveChangeRejected(t *testing.T) {
	fx := setupReviewFixture(t, 6)
	ctx := context.Background()
	if _, err := fx.svc.Apply(ctx, fx.recordID, fx.student, "李同学", "理由"); err != nil {
		t.Fatalf("Apply() error = %v", err)
	}
	// 找到客观题 question_id
	rec, _ := fx.recRepo.FindByID(ctx, fx.recordID)
	var objectiveID string
	for _, q := range rec.Questions {
		if constants.IsObjectiveQuestion(q.Type) {
			objectiveID = q.QuestionID.Hex()
		}
	}
	_, err := fx.svc.Complete(ctx, fx.recordID, fx.teacher, "王老师", dto.CompleteReviewRequest{
		Decision:       constants.ReviewStatusAdjusted,
		TeacherComment: "试图改客观题",
		Items:          []dto.ReviewGradeItem{{QuestionID: objectiveID, Score: 0}},
	})
	if err == nil {
		t.Fatal("复核改客观题应被拒绝")
	}
	// 拒绝后复核仍为待复核、成绩未变
	again, _ := fx.recRepo.FindByID(ctx, fx.recordID)
	if again.Review.Status != constants.ReviewStatusPending || again.FinalScore != 5 {
		t.Fatal("非法复核写入后数据应保持原样")
	}
	// 非法操作不产生系统审计（仍只有申请 1 条）
	if fx.sink.len() != 1 {
		t.Fatalf("非法复核不应写系统审计，实际 %d 条", fx.sink.len())
	}
}

func TestReviewCommentRequired(t *testing.T) {
	fx := setupReviewFixture(t, 6)
	ctx := context.Background()
	// 申请理由必填
	if _, err := fx.svc.Apply(ctx, fx.recordID, fx.student, "李同学", "   "); err == nil {
		t.Fatal("申请理由为空应拒绝")
	}
	if _, err := fx.svc.Apply(ctx, fx.recordID, fx.student, "李同学", "正当理由"); err != nil {
		t.Fatalf("Apply() error = %v", err)
	}
	// 复核意见必填
	if _, err := fx.svc.Complete(ctx, fx.recordID, fx.teacher, "王老师", dto.CompleteReviewRequest{
		Decision: constants.ReviewStatusRejected, TeacherComment: "  ",
	}); err == nil {
		t.Fatal("复核意见为空应拒绝")
	}
}

func TestReviewConcurrentDecideConflict(t *testing.T) {
	fx := setupReviewFixture(t, 6)
	ctx := context.Background()
	applied, err := fx.svc.Apply(ctx, fx.recordID, fx.student, "李同学", "理由")
	if err != nil {
		t.Fatalf("Apply() error = %v", err)
	}
	baseVersion := applied.Version

	// 第一位教师基于当前版本成功裁定（模拟 SaveIfVersion 内部做 version+1）
	first, err := fx.svc.Complete(ctx, fx.recordID, fx.teacher, "王老师", dto.CompleteReviewRequest{
		Decision: constants.ReviewStatusRejected, TeacherComment: "第一位已处理",
	})
	if err != nil {
		t.Fatalf("first Complete() error = %v", err)
	}
	if first.Version != baseVersion+1 {
		t.Fatalf("version=%d want %d", first.Version, baseVersion+1)
	}

	// 第二位教师持有过期版本直接走仓储条件写，应冲突，且数据保持第一次结果
	stale, _ := fx.recRepo.FindByID(ctx, fx.recordID)
	stale.Version = baseVersion // 回退版本号，模拟过期读
	stale.UpdatedAt = time.Now()
	if err := fx.recRepo.SaveIfVersion(ctx, stale, baseVersion); err == nil {
		t.Fatal("并发复核条件写应冲突")
	}
	cur, _ := fx.recRepo.FindByID(ctx, fx.recordID)
	if cur.Review.TeacherComment != "第一位已处理" || cur.Review.Status != constants.ReviewStatusRejected {
		t.Fatal("并发冲突后第一次复核结果应保持原样")
	}
}

func TestReviewStudentCannotAccessOthers(t *testing.T) {
	fx := setupReviewFixture(t, 6)
	ctx := context.Background()
	other := primitive.NewObjectID()
	if _, err := fx.svc.GetForViewer(ctx, fx.recordID, other, constants.RoleStudent); err == nil {
		t.Fatal("学生不能读取他人复核记录")
	}
	if _, err := fx.svc.Apply(ctx, fx.recordID, other, "路人甲", "替别人申请"); err == nil {
		t.Fatal("学生不能对他人记录发起复核")
	}
	// 教师可读
	if _, err := fx.svc.GetForViewer(ctx, fx.recordID, fx.teacher, constants.RoleTeacher); err != nil {
		t.Fatalf("教师应可读: %v", err)
	}
}

func TestReviewListFilters(t *testing.T) {
	fx := setupReviewFixture(t, 6)
	ctx := context.Background()
	if _, err := fx.svc.Apply(ctx, fx.recordID, fx.student, "李同学", "待处理"); err != nil {
		t.Fatalf("Apply() error = %v", err)
	}
	pending, total, err := fx.svc.ListPending(ctx, []string{constants.ReviewStatusPending}, 1, 20)
	if err != nil {
		t.Fatalf("ListPending() error = %v", err)
	}
	if total != 1 || len(pending) != 1 {
		t.Fatalf("待复核列表 total=%d len=%d", total, len(pending))
	}
	mine, mineTotal, err := fx.svc.ListMine(ctx, fx.student, nil, 1, 20)
	if err != nil {
		t.Fatalf("ListMine() error = %v", err)
	}
	if mineTotal != 1 || len(mine) != 1 {
		t.Fatalf("我的复核 total=%d len=%d", mineTotal, len(mine))
	}
	// 无复核记录不会出现在列表中（确保 review 字段过滤正确）
	all, _, _ := fx.svc.ListMine(ctx, primitive.NewObjectID(), nil, 1, 20)
	if len(all) != 0 {
		t.Fatal("其他学生不应看到该复核")
	}
}

// 复核改分后教师再次批改：复核快照与审计轨迹必须保留，及格结果随新批改按原及格线更新。
func TestReviewSnapshotSurvivesRegrade(t *testing.T) {
	fx := setupReviewFixture(t, 6)
	ctx := context.Background()
	if _, err := fx.svc.Apply(ctx, fx.recordID, fx.student, "李同学", "申请复核"); err != nil {
		t.Fatalf("apply: %v", err)
	}
	if _, err := fx.svc.Complete(ctx, fx.recordID, fx.teacher, "王老师", dto.CompleteReviewRequest{
		Decision: constants.ReviewStatusAdjusted, TeacherComment: "更正为满分",
		Items: []dto.ReviewGradeItem{{QuestionID: fx.fillQID.Hex(), Score: 5, Comment: "正确"}},
	}); err != nil {
		t.Fatalf("decide: %v", err)
	}
	// 复核后教师再次批改（主观题改为 3 分，总分 8，仍及格）
	reg, err := fx.recSvc.Grade(ctx, fx.recordID,
		[]dto.GradeItem{{QuestionID: fx.fillQID.Hex(), Score: 3, Comment: "复核后再核"}}, "t@example.com")
	if err != nil {
		t.Fatalf("regrade: %v", err)
	}
	if reg.FinalScore != 8 {
		t.Fatalf("重新批改后总分=%v，期望 8", reg.FinalScore)
	}
	if !reg.Passed {
		t.Fatal("总分 8 / 及格线 6 应及格")
	}
	if reg.Review == nil || reg.Review.Status != constants.ReviewStatusAdjusted {
		t.Fatal("复核记录应保留")
	}
	if len(reg.Review.Changes) != 1 || reg.Review.Changes[0].ScoreAfter != 5 {
		t.Fatal("复核改分快照应保留 0→5")
	}
	if len(reg.Review.AuditTrail) != 2 {
		t.Fatalf("复核审计轨迹应保留 2 条，实际 %d", len(reg.Review.AuditTrail))
	}
}
