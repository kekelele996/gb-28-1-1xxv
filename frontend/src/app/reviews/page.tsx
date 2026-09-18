'use client';
import { useCallback, useEffect, useState } from 'react';
import { useRouter } from 'next/navigation';
import { recordApi } from '@/api/record';
import { DataTable, type Column } from '@/components/DataTable';
import { Pagination } from '@/components/Pagination';
import { StatusBadge } from '@/components/StatusBadge';
import { formatDateTime, reviewStatusColor, reviewStatusText } from '@/utils/format';
import { REVIEW_STATUS } from '@/constants';
import type { ExamRecord } from '@/types';

export default function PendingReviewsPage() {
  const router = useRouter();
  const [rows, setRows] = useState<ExamRecord[]>([]);
  const [total, setTotal] = useState(0);
  const [loading, setLoading] = useState(false);
  const [page, setPage] = useState(1);

  const load = useCallback(async () => {
    setLoading(true);
    try {
      const res = await recordApi.pendingReviews({ page, page_size: 10 });
      setRows(res.list);
      setTotal(res.total);
    } finally {
      setLoading(false);
    }
  }, [page]);

  useEffect(() => {
    load();
  }, [load]);

  const columns: Column<ExamRecord>[] = [
    { key: 'exam_title', title: '考试', render: (r) => <span className="font-medium">{r.exam_title}</span> },
    { key: 'student_name', title: '学生', render: (r) => <span>{r.student_name}</span> },
    { key: 'final_score', title: '当前总分', render: (r) => <span>{r.final_score}</span> },
    {
      key: 'review',
      title: '复核状态',
      render: (r) => (
        <StatusBadge
          text={reviewStatusText(r.review?.status ?? '')}
          color={reviewStatusColor(r.review?.status ?? REVIEW_STATUS.PENDING)}
        />
      ),
    },
    { key: 'requested_at', title: '申请时间', render: (r) => <span className="text-xs">{formatDateTime(r.review?.requested_at)}</span> },
    {
      key: 'actions',
      title: '操作',
      render: (r) => (
        <button onClick={() => router.push(`/records/review?recordId=${r.id}`)} className="text-brand-600 hover:underline">
          去复核
        </button>
      ),
    },
  ];

  return (
    <div className="space-y-4">
      <h1 className="text-xl font-bold text-gray-800">待复核成绩</h1>
      <p className="text-sm text-gray-500">成绩发布后 24 小时内学生发起的复核；待复核期间成绩已锁定，请尽快处理。</p>
      <DataTable columns={columns} rows={rows} loading={loading} emptyTitle="暂无待复核成绩" />
      <Pagination page={page} pageSize={10} total={total} onChange={setPage} />
    </div>
  );
}
