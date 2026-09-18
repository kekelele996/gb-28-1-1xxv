'use client';
import { useCallback, useEffect, useMemo, useState } from 'react';
import { useRouter } from 'next/navigation';
import { useAuth } from '@/hooks/useAuth';
import { reviewApi } from '@/api/review';
import { DataTable, type Column } from '@/components/DataTable';
import { Pagination } from '@/components/Pagination';
import { StatusBadge } from '@/components/StatusBadge';
import { formatDateTime, reviewStatusColor, reviewStatusText } from '@/utils/format';
import type { ScoreReview } from '@/types';

export default function ReviewsPage() {
  const router = useRouter();
  const { isTeacher, isAdmin } = useAuth();
  const teacherView = isTeacher || isAdmin;

  const [list, setList] = useState<(ScoreReview & { id: string })[]>([]);
  const [total, setTotal] = useState(0);
  const [loading, setLoading] = useState(false);
  const [page, setPage] = useState(1);
  const [status, setStatus] = useState('');

  const reload = useCallback(async () => {
    setLoading(true);
    try {
      const res = teacherView
        ? await reviewApi.list({ status, page, page_size: 10 })
        : await reviewApi.mine({ status, page, page_size: 10 });
      // DataTable 以 id 作为行键，复核记录以 record_id 标识
      setList(res.list.map((r) => ({ ...r, id: r.record_id })));
      setTotal(res.total);
    } catch (err) {
      alert((err as Error).message);
    } finally {
      setLoading(false);
    }
  }, [teacherView, status, page]);

  useEffect(() => {
    reload();
  }, [reload]);

  const columns = useMemo<Column<ScoreReview & { id: string }>[]>(() => {
    const base: Column<ScoreReview & { id: string }>[] = [
      { key: 'exam_title', title: '考试', render: (r) => <span className="font-medium">{r.exam_title}</span> },
    ];
    if (teacherView) {
      base.push({ key: 'student_name', title: '学生', render: (r) => <span>{r.student_name}</span> });
    }
    base.push(
      { key: 'reason', title: '申请理由', render: (r) => <span className="line-clamp-1 text-xs text-gray-600">{r.reason}</span> },
      {
        key: 'score',
        title: '分数（前→后）',
        render: (r) => (
          <span className="text-sm">
            {r.final_score_before}
            {r.status === 'adjusted' && r.final_score_after !== r.final_score_before ? (
              <span className="ml-1 text-green-600">→ {r.final_score_after}</span>
            ) : null}
          </span>
        ),
      },
      { key: 'status', title: '复核状态', render: (r) => <StatusBadge text={reviewStatusText(r.status)} color={reviewStatusColor(r.status)} /> },
      { key: 'applied_at', title: '申请时间', render: (r) => <span className="text-xs">{formatDateTime(r.applied_at)}</span> },
      { key: 'decided_at', title: '复核时间', render: (r) => <span className="text-xs">{r.status === 'pending' ? '-' : formatDateTime(r.decided_at)}</span> },
      {
        key: 'actions',
        title: '操作',
        render: (r) => (
          <div className="flex gap-2">
            <button
              onClick={() => router.push(`/records/review?recordId=${r.record_id}`)}
              className="text-brand-600 hover:underline"
            >
              {teacherView && r.status === 'pending' ? '去复核' : '查看详情'}
            </button>
          </div>
        ),
      },
    );
    return base;
  }, [teacherView, router]);

  return (
    <div className="space-y-4">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <h1 className="text-xl font-bold text-gray-800">{teacherView ? '成绩复核处理' : '我的成绩复核'}</h1>
        <div className="flex items-center gap-2">
          <select
            value={status}
            onChange={(e) => {
              setStatus(e.target.value);
              setPage(1);
            }}
            className="rounded-lg border border-gray-300 px-2 py-1.5 text-sm"
          >
            <option value="">全部状态</option>
            <option value="pending">待复核</option>
            <option value="adjusted">已调整</option>
            <option value="rejected">维持原判</option>
          </select>
        </div>
      </div>
      {teacherView && (
        <p className="text-xs text-gray-500">待复核期间成绩已锁定，学生提交后教师无法重复批改；复核只能调整主观题分值。</p>
      )}
      <DataTable
        columns={columns}
        rows={list}
        loading={loading}
        emptyTitle={teacherView ? '暂无成绩复核申请' : '还没有发起过成绩复核'}
      />
      <Pagination page={page} pageSize={10} total={total} onChange={setPage} />
    </div>
  );
}
