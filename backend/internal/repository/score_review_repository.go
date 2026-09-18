package repository

import (
	"context"
	"fmt"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/onlineexam/onlineexam/internal/model"
)

// ScoreReviewRepository 成绩复核仓储接口。
// 复核记录内嵌于 exam_records 集合（review 字段），因此这里只提供面向复核场景的查询。
// 写入（申请/裁定）统一走 ExamRecordRepository.SaveIfVersion 乐观锁条件写，保证复核记录与成绩同文档一次落盘。
type ScoreReviewRepository interface {
	// ListByStatus 按复核状态分页查询（教师/管理员的待复核列表）。
	ListByStatus(ctx context.Context, statuses []string, page, pageSize int64) ([]*model.ExamRecord, int64, error)
	// ListByStudent 查询某学生的全部复核记录（学生回读自己的申请与复核结果）。
	ListByStudent(ctx context.Context, studentID primitive.ObjectID, statuses []string, page, pageSize int64) ([]*model.ExamRecord, int64, error)
}

// MongoScoreReviewRepository MongoDB 成绩复核仓储实现（操作 exam_records 集合的 review 字段）。
type MongoScoreReviewRepository struct {
	coll *mongo.Collection
}

// NewMongoScoreReviewRepository 构造成绩复核仓储。
func NewMongoScoreReviewRepository(db *mongo.Database) *MongoScoreReviewRepository {
	return &MongoScoreReviewRepository{coll: db.Collection("exam_records")}
}

func (r *MongoScoreReviewRepository) ListByStatus(ctx context.Context, statuses []string, page, pageSize int64) ([]*model.ExamRecord, int64, error) {
	return r.list(ctx, bson.M{"review.status": bson.M{"$in": statuses}}, page, pageSize)
}

func (r *MongoScoreReviewRepository) ListByStudent(ctx context.Context, studentID primitive.ObjectID, statuses []string, page, pageSize int64) ([]*model.ExamRecord, int64, error) {
	filter := bson.M{"review.student_id": studentID}
	if len(statuses) > 0 {
		filter["review.status"] = bson.M{"$in": statuses}
	}
	return r.list(ctx, filter, page, pageSize)
}

func (r *MongoScoreReviewRepository) list(ctx context.Context, filter bson.M, page, pageSize int64) ([]*model.ExamRecord, int64, error) {
	total, err := r.coll.CountDocuments(ctx, filter)
	if err != nil {
		return nil, 0, fmt.Errorf("count score reviews: %w", err)
	}
	opts := options.Find().
		SetSkip((page - 1) * pageSize).
		SetLimit(pageSize).
		SetSort(bson.M{"review.applied_at": -1})
	cur, err := r.coll.Find(ctx, filter, opts)
	if err != nil {
		return nil, 0, fmt.Errorf("list score reviews: %w", err)
	}
	defer func() { _ = cur.Close(ctx) }()
	var recs []*model.ExamRecord
	if err := cur.All(ctx, &recs); err != nil {
		return nil, 0, fmt.Errorf("decode score reviews: %w", err)
	}
	return recs, total, nil
}
