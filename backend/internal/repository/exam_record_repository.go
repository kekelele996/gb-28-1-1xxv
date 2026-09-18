package repository

import (
	"context"
	"errors"
	"fmt"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/onlineexam/onlineexam/internal/model"
)

// ExamRecordRepository 考试记录仓储接口。
type ExamRecordRepository interface {
	Create(ctx context.Context, r *model.ExamRecord) error
	Update(ctx context.Context, r *model.ExamRecord) error
	// ReplaceIf 原子条件替换：仅当 filter 全部满足时才整体替换文档。
	// 返回 matched=false 表示前置条件已被并发请求改变（重复提交/并发复核），
	// 此时文档保持原样，调用方必须返回冲突错误，不得做任何二次写入。
	ReplaceIf(ctx context.Context, r *model.ExamRecord, filter bson.M) (matched bool, err error)
	FindByID(ctx context.Context, id primitive.ObjectID) (*model.ExamRecord, error)
	FindActiveByExamAndStudent(ctx context.Context, examID, studentID primitive.ObjectID) (*model.ExamRecord, error)
	List(ctx context.Context, filter bson.M, page, pageSize int64) ([]*model.ExamRecord, int64, error)
	ListAll(ctx context.Context, filter bson.M) ([]*model.ExamRecord, error)
	// ListPendingReviews 分页查询待复核（review.status=pending）记录，按复核申请时间升序。
	ListPendingReviews(ctx context.Context, page, pageSize int64) ([]*model.ExamRecord, int64, error)
	CountByExamAndStatus(ctx context.Context, examID primitive.ObjectID, statuses []string) (int64, error)
}

// MongoExamRecordRepository MongoDB 考试记录仓储实现。
type MongoExamRecordRepository struct {
	coll *mongo.Collection
}

// NewMongoExamRecordRepository 构造考试记录仓储。
func NewMongoExamRecordRepository(db *mongo.Database) *MongoExamRecordRepository {
	return &MongoExamRecordRepository{coll: db.Collection("exam_records")}
}

func (r *MongoExamRecordRepository) Create(ctx context.Context, rec *model.ExamRecord) error {
	_, err := r.coll.InsertOne(ctx, rec)
	if err != nil {
		return fmt.Errorf("create exam record: %w", err)
	}
	return nil
}

func (r *MongoExamRecordRepository) Update(ctx context.Context, rec *model.ExamRecord) error {
	res, err := r.coll.ReplaceOne(ctx, bson.M{"_id": rec.ID}, rec)
	if err != nil {
		return fmt.Errorf("update exam record: %w", err)
	}
	if res.MatchedCount == 0 {
		return fmt.Errorf("update exam record: %w", ErrNotFound)
	}
	return nil
}

// ReplaceIf 原子条件替换：filter 与 _id 合并后执行 ReplaceOne。
// MongoDB 单文档写入天然原子；条件不满足时 MatchedCount=0，原文档不发生任何改变，
// 从而保证「重复提交 / 并发复核 / 写入失败时，记录、成绩和审计保持原样」。
func (r *MongoExamRecordRepository) ReplaceIf(ctx context.Context, rec *model.ExamRecord, filter bson.M) (bool, error) {
	cond := bson.M{"_id": rec.ID}
	for k, v := range filter {
		cond[k] = v
	}
	res, err := r.coll.ReplaceOne(ctx, cond, rec)
	if err != nil {
		return false, fmt.Errorf("replace exam record if: %w", err)
	}
	return res.MatchedCount > 0, nil
}

func (r *MongoExamRecordRepository) FindByID(ctx context.Context, id primitive.ObjectID) (*model.ExamRecord, error) {
	var rec model.ExamRecord
	if err := r.coll.FindOne(ctx, bson.M{"_id": id}).Decode(&rec); err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, fmt.Errorf("find exam record by id: %w", ErrNotFound)
		}
		return nil, fmt.Errorf("find exam record by id: %w", err)
	}
	return &rec, nil
}

func (r *MongoExamRecordRepository) FindActiveByExamAndStudent(ctx context.Context, examID, studentID primitive.ObjectID) (*model.ExamRecord, error) {
	var rec model.ExamRecord
	err := r.coll.FindOne(ctx, bson.M{
		"exam_id":    examID,
		"student_id": studentID,
		"status":     modelStatusInProgress(),
	}).Decode(&rec)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, fmt.Errorf("find active exam record: %w", ErrNotFound)
		}
		return nil, fmt.Errorf("find active exam record: %w", err)
	}
	return &rec, nil
}

func (r *MongoExamRecordRepository) List(ctx context.Context, filter bson.M, page, pageSize int64) ([]*model.ExamRecord, int64, error) {
	total, err := r.coll.CountDocuments(ctx, filter)
	if err != nil {
		return nil, 0, fmt.Errorf("count exam records: %w", err)
	}
	opts := options.Find().
		SetSkip((page - 1) * pageSize).
		SetLimit(pageSize).
		SetSort(bson.M{"started_at": -1})
	cur, err := r.coll.Find(ctx, filter, opts)
	if err != nil {
		return nil, 0, fmt.Errorf("list exam records: %w", err)
	}
	defer func() { _ = cur.Close(ctx) }()
	var recs []*model.ExamRecord
	if err := cur.All(ctx, &recs); err != nil {
		return nil, 0, fmt.Errorf("decode exam records: %w", err)
	}
	return recs, total, nil
}

func (r *MongoExamRecordRepository) ListAll(ctx context.Context, filter bson.M) ([]*model.ExamRecord, error) {
	cur, err := r.coll.Find(ctx, filter)
	if err != nil {
		return nil, fmt.Errorf("list all exam records: %w", err)
	}
	defer func() { _ = cur.Close(ctx) }()
	var recs []*model.ExamRecord
	if err := cur.All(ctx, &recs); err != nil {
		return nil, fmt.Errorf("decode all exam records: %w", err)
	}
	return recs, nil
}

// ListPendingReviews 查询待复核记录（review.status=pending），按申请时间升序（先申请先处理）。
func (r *MongoExamRecordRepository) ListPendingReviews(ctx context.Context, page, pageSize int64) ([]*model.ExamRecord, int64, error) {
	filter := bson.M{"review.status": modelReviewStatusPending()}
	total, err := r.coll.CountDocuments(ctx, filter)
	if err != nil {
		return nil, 0, fmt.Errorf("count pending reviews: %w", err)
	}
	opts := options.Find().
		SetSkip((page - 1) * pageSize).
		SetLimit(pageSize).
		SetSort(bson.M{"review.requested_at": 1})
	cur, err := r.coll.Find(ctx, filter, opts)
	if err != nil {
		return nil, 0, fmt.Errorf("list pending reviews: %w", err)
	}
	defer func() { _ = cur.Close(ctx) }()
	var recs []*model.ExamRecord
	if err := cur.All(ctx, &recs); err != nil {
		return nil, 0, fmt.Errorf("decode pending reviews: %w", err)
	}
	return recs, total, nil
}

func (r *MongoExamRecordRepository) CountByExamAndStatus(ctx context.Context, examID primitive.ObjectID, statuses []string) (int64, error) {
	filter := bson.M{"exam_id": examID}
	if len(statuses) > 0 {
		filter["status"] = bson.M{"$in": statuses}
	}
	count, err := r.coll.CountDocuments(ctx, filter)
	if err != nil {
		return 0, fmt.Errorf("count exam records by status: %w", err)
	}
	return count, nil
}

// modelStatusInProgress 避免 repository 直接依赖 constants 的循环，这里硬编码状态值。
// 注意：该状态枚举同时在 constants/enums.go、service 状态机、formatters、前端 constants 中出现。
func modelStatusInProgress() string {
	return "in_progress"
}

// modelReviewStatusPending 成绩复核「待复核」状态值（与 constants.ReviewStatusPending 对应，
// 同样为避免 constants 循环依赖而硬编码）。
func modelReviewStatusPending() string {
	return "pending"
}
