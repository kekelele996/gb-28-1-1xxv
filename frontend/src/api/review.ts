import { request, buildQuery } from '@/utils/request';
import type { ExamRecord, PageResult, ScoreReview } from '@/types';

export interface ReviewGradeItem {
  question_id: string;
  score: number;
  comment?: string;
}

export interface CompleteReviewInput {
  decision: 'adjusted' | 'rejected';
  teacher_comment: string;
  items?: ReviewGradeItem[];
}

export const reviewApi = {
  // 学生：对本人已批改记录发起一次复核（成绩发布后 24h 内、理由必填）
  apply(recordId: string, reason: string) {
    return request<ExamRecord>(`/exam-records/${recordId}/review`, {
      method: 'POST',
      body: JSON.stringify({ reason }),
    });
  },
  // 按角色读取复核详情（学生限本人）
  get(recordId: string) {
    return request<ExamRecord>(`/exam-records/${recordId}/review`);
  },
  // 教师：复核裁定（只能改主观题；重算总分、按原及格线更新结果）
  decide(recordId: string, input: CompleteReviewInput) {
    return request<ExamRecord>(`/exam-records/${recordId}/review/decide`, {
      method: 'POST',
      body: JSON.stringify(input),
    });
  },
  // 教师/管理员：复核列表
  list(query: { status?: string; page?: number; page_size?: number }) {
    return request<PageResult<ScoreReview>>(`/score-reviews${buildQuery({ ...query })}`);
  },
  // 学生：我的复核申请
  mine(query: { status?: string; page?: number; page_size?: number }) {
    return request<PageResult<ScoreReview>>(`/score-reviews/mine${buildQuery({ ...query })}`);
  },
};
