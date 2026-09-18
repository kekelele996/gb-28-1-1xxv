'use client';
import { Suspense, useCallback, useEffect, useMemo, useState } from 'react';
import { useSearchParams } from 'next/navigation';
import { useAuth } from '@/hooks/useAuth';
import { recordApi } from '@/api/record';
import { reviewApi } from '@/api/review';
import { wrongBookApi } from '@/api/wrongBook';
import { Modal } from '@/components/Modal';
import { QuestionTypeBadge, StatusBadge } from '@/components/StatusBadge';
import {
  answerResultText,
  formatDateTime,
  recordStatusColor,
  recordStatusText,
  reviewDeadline,
  reviewStatusColor,
  reviewStatusText,
} from '@/utils/format';
import { ANSWER_RESULT, REVIEW_STATUS } from '@/constants';
import type { ExamRecord } from '@/types';

function Review() {
  const params = useSearchParams();
  const recordId = params.get('recordId') ?? '';
  const { isTeacher, isAdmin, isStudent } = useAuth();
  const canGrade = isTeacher || isAdmin;

  const [record, setRecord] = useState<ExamRecord | null>(null);
  const [scores, setScores] = useState<Record<string, number>>({});
  const [comments, setComments] = useState<Record<string, string>>({});
  const [grading, setGrading] = useState(false);
  const [adding, setAdding] = useState(false);

  // 学生申请复核
  const [applyOpen, setApplyOpen] = useState(false);
  const [reason, setReason] = useState('');
  const [applying, setApplying] = useState(false);

  // 教师复核裁定
  const [reviewScores, setReviewScores] = useState<Record<string, number>>({});
  const [reviewComments, setReviewComments] = useState<Record<string, string>>({});
  const [decision, setDecision] = useState<'adjusted' | 'rejected'>('adjusted');
  const [teacherComment, setTeacherComment] = useState('');
  const [deciding, setDeciding] = useState(false);

  const load = useCallback(async () => {
    if (!recordId) return;
    const r = await recordApi.get(recordId);
    setRecord(r);
    const sc: Record<string, number> = {};
    const cm: Record<string, string> = {};
    const rsc: Record<string, number> = {};
    r.questions.forEach((q) => {
      if (q.subjective_score) sc[q.question_id] = q.subjective_score;
      if (q.comment) cm[q.question_id] = q.comment;
      if (q.type === 'fill' || q.type === 'short') {
        rsc[q.question_id] = q.got_score ?? 0;
      }
    });
    setScores(sc);
    setComments(cm);
    setReviewScores(rsc);
  }, [recordId]);

  useEffect(() => {
    load();
  }, [load]);

  const subjectiveQuestions = useMemo(
    () => (record?.questions ?? []).filter((q) => q.type === 'fill' || q.type === 'short'),
    [record],
  );

  if (!record) return <div className="p-10 text-center text-gray-400">加载中…</div>;

  const review = record.review ?? null;
  const reviewPending = review?.status === REVIEW_STATUS.PENDING;
  const windowOpen =
    record.status === 'graded' && !!record.graded_at && new Date(record.graded_at).getTime() + 24 * 3600 * 1000 > Date.now();
  const canApply = isStudent && record.status === 'graded' && !review && windowOpen;

  const resultColor = (r: string) => {
    switch (r) {
      case ANSWER_RESULT.CORRECT: return 'green';
      case ANSWER_RESULT.WRONG: return 'red';
      case ANSWER_RESULT.PARTIAL: return 'orange';
      default: return 'gray';
    }
  };

  const onGrade = async () => {
    setGrading(true);
    try {
      const grades = subjectiveQuestions.map((q) => ({
        question_id: q.question_id,
        score: scores[q.question_id] ?? 0,
        comment: comments[q.question_id] ?? '',
      }));
      await recordApi.grade(record.id, grades);
      alert('批改完成');
      load();
    } catch (err) {
      alert((err as Error).message);
    } finally {
      setGrading(false);
    }
  };

  const onApply = async () => {
    if (!reason.trim()) {
      alert('请填写复核理由');
      return;
    }
    setApplying(true);
    try {
      await reviewApi.apply(record.id, reason.trim());
      alert('复核申请已提交，等待教师处理');
      setApplyOpen(false);
      setReason('');
      load();
    } catch (err) {
      alert((err as Error).message);
    } finally {
      setApplying(false);
    }
  };

  const onDecide = async () => {
    if (!teacherComment.trim()) {
      alert('请填写复核意见');
      return;
    }
    const changedItems = subjectiveQuestions
      .filter((q) => (reviewScores[q.question_id] ?? 0) !== (q.got_score ?? 0))
      .map((q) => ({
        question_id: q.question_id,
        score: reviewScores[q.question_id] ?? 0,
        comment: reviewComments[q.question_id] ?? '',
      }));
    if (decision === 'adjusted' && changedItems.length === 0) {
      alert('裁定为“调整分数”时至少需要修改一道主观题分值；无修改请选择“维持原判”');
      return;
    }
    setDeciding(true);
    try {
      await reviewApi.decide(record.id, {
        decision,
        teacher_comment: teacherComment.trim(),
        items: decision === 'adjusted' ? changedItems : undefined,
      });
      alert('复核已完成');
      setTeacherComment('');
      load();
    } catch (err) {
      alert((err as Error).message);
    } finally {
      setDeciding(false);
    }
  };

  const onAddWrong = async (q: { question_id: string }) => {
    setAdding(true);
    try {
      await wrongBookApi.add({ question_id: q.question_id, exam_id: record.exam_id, exam_record_id: record.id });
      alert('已加入错题本');
    } catch (err) {
      alert((err as Error).message);
    } finally {
      setAdding(false);
    }
  };

  return (
    <div className="space-y-4">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <div>
          <h1 className="text-xl font-bold text-gray-800">{record.exam_title} · 答卷详情</h1>
          <p className="mt-1 text-sm text-gray-500">
            学生 {record.student_name} · 客观题 {record.objective_score} 分 · 主观题 {record.subjective_score} 分 · 最终{' '}
            <span className="font-semibold">{record.final_score || '-'}</span> 分
            {record.status === 'graded' && (
              <span className={`ml-2 ${record.passed ? 'text-green-600' : 'text-red-600'}`}>
                （及格线 {record.pass_score}，{record.passed ? '及格' : '不及格'}）
              </span>
            )}
            {' '}· 切屏 {record.cheat_count} 次
          </p>
          <p className="text-xs text-gray-400">
            开始 {formatDateTime(record.started_at)}
            {record.graded_at ? ` · 成绩发布 ${formatDateTime(record.graded_at)}` : ''}
          </p>
        </div>
        <div className="flex items-center gap-2">
          <StatusBadge text={recordStatusText(record.status)} color={recordStatusColor(record.status)} />
          {review && <StatusBadge text={reviewStatusText(review.status)} color={reviewStatusColor(review.status)} />}
        </div>
      </div>

      {/* 成绩锁定提示：待复核期间教师不能重复批改 */}
      {reviewPending && (
        <div className="rounded-xl border border-orange-300 bg-orange-50 p-4 text-sm text-orange-700">
          该成绩正在复核处理中，成绩已锁定，教师无法重复批改；学生于 {formatDateTime(review?.applied_at)} 发起复核。
        </div>
      )}

      {/* 学生：24h 内可发起一次复核 */}
      {canApply && (
        <div className="flex flex-wrap items-center justify-between gap-3 rounded-xl border border-brand-200 bg-brand-50 p-4">
          <div className="text-sm text-brand-800">
            对成绩有异议？可在成绩发布后 24 小时内发起一次复核。
            <span className="ml-2 text-xs text-brand-500">{reviewDeadline(record.graded_at)}</span>
          </div>
          <button
            onClick={() => setApplyOpen(true)}
            className="rounded-lg bg-brand-600 px-4 py-2 text-sm text-white hover:bg-brand-700"
          >
            申请成绩复核
          </button>
        </div>
      )}
      {isStudent && record.status === 'graded' && !review && !windowOpen && (
        <div className="rounded-xl border border-gray-200 bg-gray-50 p-4 text-sm text-gray-500">
          成绩发布已超过 24 小时，复核窗口已关闭。
        </div>
      )}

      {/* 复核记录回读：申请、复核状态、修改前后分数、复核意见、审计轨迹 */}
      {review && (
        <section className="rounded-xl border border-gray-200 bg-white p-5">
          <div className="flex flex-wrap items-center justify-between gap-2">
            <h2 className="font-semibold text-gray-800">成绩复核</h2>
            <StatusBadge text={reviewStatusText(review.status)} color={reviewStatusColor(review.status)} />
          </div>
          <dl className="mt-3 grid gap-x-8 gap-y-2 text-sm sm:grid-cols-2">
            <div><dt className="inline text-gray-400">申请理由：</dt><dd className="inline">{review.reason}</dd></div>
            <div><dt className="inline text-gray-400">申请时间：</dt><dd className="inline">{formatDateTime(review.applied_at)}</dd></div>
            <div>
              <dt className="inline text-gray-400">总分：</dt>
              <dd className="inline">
                {review.final_score_before}
                {review.status === REVIEW_STATUS.ADJUSTED && review.final_score_after !== review.final_score_before ? (
                  <span className="ml-1 font-semibold text-green-600">→ {review.final_score_after}</span>
                ) : null}
              </dd>
            </div>
            <div>
              <dt className="inline text-gray-400">是否及格：</dt>
              <dd className="inline">
                {review.passed_before ? '及格' : '不及格'}
                {review.passed_after !== null && review.passed_after !== review.passed_before ? (
                  <span className={`ml-1 font-semibold ${review.passed_after ? 'text-green-600' : 'text-red-600'}`}>
                    → {review.passed_after ? '及格' : '不及格'}
                  </span>
                ) : null}
                <span className="ml-1 text-xs text-gray-400">（按原及格线 {review.pass_score}）</span>
              </dd>
            </div>
            {review.teacher_name && (
              <div><dt className="inline text-gray-400">复核教师：</dt><dd className="inline">{review.teacher_name}</dd></div>
            )}
            {review.decided_at && (
              <div><dt className="inline text-gray-400">复核时间：</dt><dd className="inline">{formatDateTime(review.decided_at)}</dd></div>
            )}
          </dl>

          {review.changes.length > 0 && (
            <div className="mt-4">
              <h3 className="text-sm font-semibold text-gray-700">改分明细</h3>
              <div className="mt-2 overflow-hidden rounded-lg border border-gray-100">
                <table className="w-full text-sm">
                  <thead className="bg-gray-50 text-xs text-gray-500">
                    <tr>
                      <th className="px-3 py-2 text-left">题目</th>
                      <th className="px-3 py-2 text-left">题型</th>
                      <th className="px-3 py-2 text-left">修改前</th>
                      <th className="px-3 py-2 text-left">修改后</th>
                      <th className="px-3 py-2 text-left">评语</th>
                    </tr>
                  </thead>
                  <tbody>
                    {review.changes.map((c) => (
                      <tr key={c.question_id} className="border-t border-gray-100">
                        <td className="max-w-[18rem] truncate px-3 py-2">{c.content}</td>
                        <td className="px-3 py-2">
                          <QuestionTypeBadge type={c.question_type} />
                        </td>
                        <td className="px-3 py-2">{c.score_before}</td>
                        <td className="px-3 py-2 font-semibold text-green-600">{c.score_after}</td>
                        <td className="max-w-[14rem] truncate px-3 py-2 text-xs text-gray-500">{c.comment_after || '-'}</td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            </div>
          )}

          {review.teacher_comment && (
            <p className="mt-3 rounded-lg bg-gray-50 p-3 text-sm">
              <span className="text-gray-400">复核意见：</span>
              {review.teacher_comment}
            </p>
          )}

          {/* 审计轨迹：申请 + 复核一次落盘 */}
          <div className="mt-4">
            <h3 className="text-sm font-semibold text-gray-700">审计记录</h3>
            <ol className="mt-2 space-y-2">
              {review.audit_trail.map((a, i) => (
                <li key={i} className="flex items-start gap-2 text-xs text-gray-500">
                  <StatusBadge text={a.action === 'review_apply' ? '申请' : '复核'} color={a.action === 'review_apply' ? 'blue' : 'purple'} />
                  <span>
                    {a.operator_name}（{a.role}）· {formatDateTime(a.created_at)} · {a.comment}
                  </span>
                </li>
              ))}
            </ol>
          </div>
        </section>
      )}

      {/* 教师：待复核裁定面板（只能改主观题） */}
      {canGrade && reviewPending && (
        <section className="rounded-xl border border-purple-200 bg-purple-50 p-5">
          <h2 className="font-semibold text-purple-800">成绩复核处理</h2>
          <div className="mt-3 space-y-3">
            {subjectiveQuestions.map((q) => (
              <div key={q.question_id} className="rounded-lg bg-white p-3">
                <p className="text-sm text-gray-800">{q.content}</p>
                <p className="mt-1 text-xs text-gray-400">
                  当前得分 {q.got_score ?? 0} / {q.score}
                </p>
                <div className="mt-2 grid gap-2 sm:grid-cols-2">
                  <input
                    type="number"
                    min={0}
                    max={q.score}
                    step={0.5}
                    value={reviewScores[q.question_id] ?? 0}
                    disabled={decision === 'rejected'}
                    onChange={(e) => setReviewScores({ ...reviewScores, [q.question_id]: Number(e.target.value) })}
                    className="rounded-lg border border-gray-300 px-3 py-1.5 text-sm disabled:bg-gray-100"
                  />
                  <input
                    value={reviewComments[q.question_id] ?? ''}
                    placeholder="本题复核评语（可选）"
                    disabled={decision === 'rejected'}
                    onChange={(e) => setReviewComments({ ...reviewComments, [q.question_id]: e.target.value })}
                    className="rounded-lg border border-gray-300 px-3 py-1.5 text-sm disabled:bg-gray-100"
                  />
                </div>
              </div>
            ))}
            <div className="flex items-center gap-4 text-sm">
              <label className="flex items-center gap-1">
                <input type="radio" checked={decision === 'adjusted'} onChange={() => setDecision('adjusted')} />
                调整分数（重算总分）
              </label>
              <label className="flex items-center gap-1">
                <input type="radio" checked={decision === 'rejected'} onChange={() => setDecision('rejected')} />
                维持原判
              </label>
            </div>
            <textarea
              value={teacherComment}
              onChange={(e) => setTeacherComment(e.target.value)}
              placeholder="复核意见（必填）"
              rows={3}
              className="w-full rounded-lg border border-gray-300 px-3 py-2 text-sm"
            />
            <div className="flex justify-end">
              <button
                onClick={onDecide}
                disabled={deciding}
                className="rounded-lg bg-purple-600 px-6 py-2 text-sm text-white hover:bg-purple-700 disabled:opacity-60"
              >
                {deciding ? '提交中…' : '提交复核结果'}
              </button>
            </div>
          </div>
        </section>
      )}

      {record.questions.map((q, i) => (
        <div key={q.question_id} className="rounded-xl border border-gray-200 bg-white p-5">
          <div className="flex items-center justify-between">
            <span className="text-sm text-gray-400">第 {i + 1} 题</span>
            <div className="flex items-center gap-2">
              <QuestionTypeBadge type={q.type} />
              <StatusBadge text={answerResultText(q.result)} color={resultColor(q.result)} />
            </div>
          </div>
          <p className="mt-2 font-medium text-gray-800">{q.content}</p>
          <p className="mt-1 text-xs text-gray-400">{q.score} 分 · 本题得分 {q.got_score || 0}</p>

          {(q.type === 'single' || q.type === 'multiple' || q.type === 'judge') && (
            <div className="mt-3 space-y-1">
              {q.type === 'judge'
                ? ['true', 'false'].map((v) => (
                    <p key={v} className="text-sm">
                      {v === 'true' ? '正确' : '错误'}
                      {q.correct_answer === v && <span className="ml-2 text-green-600">✓ 正确答案</span>}
                      {q.user_answer === v && <span className={`ml-2 ${q.user_answer === q.correct_answer ? 'text-green-600' : 'text-red-600'}`}>我的答案</span>}
                    </p>
                  ))
                : (q.options ?? []).map((o) => (
                    <p key={o.key} className="text-sm">
                      {o.key}. {o.text}
                      {q.correct_answer.split(',').includes(o.key) && <span className="ml-2 text-green-600">✓ 正确答案</span>}
                      {(q.user_answer ?? '').split(',').includes(o.key) && (
                        <span className={`ml-2 ${(q.correct_answer ?? '').split(',').includes(o.key) ? 'text-green-600' : 'text-red-600'}`}>我的答案</span>
                      )}
                    </p>
                  ))}
            </div>
          )}

          {q.type === 'fill' || q.type === 'short' ? (
            <div className="mt-3 rounded-lg bg-gray-50 p-3 text-sm">
              <p><span className="text-gray-500">我的答案：</span>{q.user_answer || '（未作答）'}</p>
              <p className="mt-1"><span className="text-gray-500">参考答案：</span>{q.correct_answer}</p>
              {canGrade && !reviewPending && (
                <div className="mt-3 grid gap-2 sm:grid-cols-2">
                  <input type="number" min={0} max={q.score} step={0.5} value={scores[q.question_id] ?? ''}
                    placeholder="给分"
                    onChange={(e) => setScores({ ...scores, [q.question_id]: Number(e.target.value) })}
                    className="rounded-lg border border-gray-300 px-3 py-1.5 text-sm" />
                  <input value={comments[q.question_id] ?? ''} placeholder="评语（可选）"
                    onChange={(e) => setComments({ ...comments, [q.question_id]: e.target.value })}
                    className="rounded-lg border border-gray-300 px-3 py-1.5 text-sm" />
                </div>
              )}
            </div>
          ) : (
            <div className="mt-2 text-sm">
              <p><span className="text-gray-500">我的答案：</span>{q.user_answer || '（未作答）'}</p>
            </div>
          )}

          <div className="mt-3 flex items-center justify-between border-t border-gray-100 pt-3">
            <p className="text-xs text-gray-400">本题得分 {q.got_score || 0}/{q.score}</p>
            {isStudent && q.result === ANSWER_RESULT.WRONG && (
              <button onClick={() => onAddWrong(q)} disabled={adding}
                className="rounded-lg border border-amber-500 px-3 py-1 text-xs text-amber-600 hover:bg-amber-50 disabled:opacity-60">
                加入错题本
              </button>
            )}
          </div>
        </div>
      ))}

      {/* 常规批改入口：待复核期间隐藏（成绩锁定） */}
      {canGrade && !reviewPending && record.status !== 'graded' && (
        <div className="flex justify-end">
          <button onClick={onGrade} disabled={grading}
            className="rounded-lg bg-brand-600 px-6 py-2 text-sm text-white hover:bg-brand-700 disabled:opacity-60">
            {grading ? '批改中…' : '保存批改'}
          </button>
        </div>
      )}

      <Modal open={applyOpen} title="申请成绩复核" onClose={() => setApplyOpen(false)}>
        <div className="space-y-3">
          <p className="text-xs text-gray-500">
            复核只能在成绩发布后 24 小时内发起一次；提交后成绩将锁定，待教师复核完成后更新。
          </p>
          <textarea
            value={reason}
            onChange={(e) => setReason(e.target.value)}
            rows={4}
            maxLength={500}
            placeholder="请填写申请理由（必填）"
            className="w-full rounded-lg border border-gray-300 px-3 py-2 text-sm"
          />
          <div className="flex justify-end gap-2">
            <button onClick={() => setApplyOpen(false)} className="rounded-lg border border-gray-300 px-4 py-2 text-sm hover:bg-gray-50">
              取消
            </button>
            <button onClick={onApply} disabled={applying || !reason.trim()}
              className="rounded-lg bg-brand-600 px-4 py-2 text-sm text-white hover:bg-brand-700 disabled:opacity-60">
              {applying ? '提交中…' : '确认提交'}
            </button>
          </div>
        </div>
      </Modal>
    </div>
  );
}

export default function ReviewPage() {
  return (
    <Suspense fallback={<div className="p-10 text-center text-gray-400">加载中…</div>}>
      <Review />
    </Suspense>
  );
}
