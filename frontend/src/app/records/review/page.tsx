'use client';
import { Suspense, useCallback, useEffect, useMemo, useState } from 'react';
import { useSearchParams } from 'next/navigation';
import { useAuth } from '@/hooks/useAuth';
import { recordApi } from '@/api/record';
import { wrongBookApi } from '@/api/wrongBook';
import { QuestionTypeBadge, StatusBadge } from '@/components/StatusBadge';
import { Modal } from '@/components/Modal';
import { answerResultText, formatDateTime, recordStatusColor, recordStatusText, reviewStatusColor, reviewStatusText, withinReviewWindow } from '@/utils/format';
import { ANSWER_RESULT, RECORD_STATUS, REVIEW_STATUS } from '@/constants';
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

  // 学生复核申请
  const [applyOpen, setApplyOpen] = useState(false);
  const [reason, setReason] = useState('');
  const [submittingReview, setSubmittingReview] = useState(false);

  // 教师复核处理
  const [reviewScores, setReviewScores] = useState<Record<string, number>>({});
  const [reviewComments, setReviewComments] = useState<Record<string, string>>({});
  const [opinion, setOpinion] = useState('');
  const [handling, setHandling] = useState(false);

  const load = useCallback(async () => {
    if (!recordId) return;
    const r = await recordApi.get(recordId);
    setRecord(r);
    const sc: Record<string, number> = {};
    const cm: Record<string, string> = {};
    const rsc: Record<string, number> = {};
    const rcm: Record<string, string> = {};
    r.questions.forEach((q) => {
      if (q.subjective_score !== undefined && q.subjective_score !== null) {
        sc[q.question_id] = q.subjective_score;
        rsc[q.question_id] = q.subjective_score;
      }
      if (q.comment) {
        cm[q.question_id] = q.comment;
        rcm[q.question_id] = q.comment;
      }
    });
    setScores(sc);
    setComments(cm);
    setReviewScores(rsc);
    setReviewComments(rcm);
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
  const isPending = review?.status === REVIEW_STATUS.PENDING;
  // 成绩锁定：待复核期间教师不能重复批改。
  const gradeLocked = isPending;
  const inWindow = withinReviewWindow(record.review_deadline);
  const canApplyReview =
    isStudent && record.status === RECORD_STATUS.GRADED && !review && inWindow;
  const windowExpired = isStudent && record.status === RECORD_STATUS.GRADED && !review && !inWindow;

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

  const onApplyReview = async () => {
    if (!reason.trim()) {
      alert('请填写复核理由');
      return;
    }
    setSubmittingReview(true);
    try {
      await recordApi.requestReview(record.id, reason.trim());
      setApplyOpen(false);
      setReason('');
      alert('复核申请已提交，待复核期间成绩锁定');
      load();
    } catch (err) {
      alert((err as Error).message);
    } finally {
      setSubmittingReview(false);
    }
  };

  const onCompleteReview = async () => {
    if (!opinion.trim()) {
      alert('请填写复核意见');
      return;
    }
    setHandling(true);
    try {
      const grades = subjectiveQuestions.map((q) => ({
        question_id: q.question_id,
        score: reviewScores[q.question_id] ?? 0,
        comment: reviewComments[q.question_id] ?? '',
      }));
      await recordApi.completeReview(record.id, { teacher_opinion: opinion.trim(), grades });
      alert('复核处理完成，总分与及格结果已更新');
      load();
    } catch (err) {
      alert((err as Error).message);
    } finally {
      setHandling(false);
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
            学生 {record.student_name} · 客观题 {record.objective_score} 分 · 主观题 {record.subjective_score} 分 ·
            最终 <span className="font-semibold">{record.final_score || '-'}</span> 分
            {record.status === RECORD_STATUS.GRADED && record.pass_score > 0 && (
              <span className={`ml-2 rounded-full px-2 py-0.5 text-xs font-medium ${record.passed ? 'bg-green-100 text-green-700' : 'bg-red-100 text-red-700'}`}>
                {record.passed ? '及格' : '不及格'}（及格线 {record.pass_score}）
              </span>
            )}
            {' '}· 切屏 {record.cheat_count} 次
          </p>
          <p className="text-xs text-gray-400">
            开始 {formatDateTime(record.started_at)}
            {record.graded_at && <span> · 成绩发布 {formatDateTime(record.graded_at)}</span>}
          </p>
        </div>
        <div className="flex items-center gap-2">
          {isPending && <StatusBadge text={reviewStatusText(REVIEW_STATUS.PENDING)} color={reviewStatusColor(REVIEW_STATUS.PENDING)} />}
          <StatusBadge text={recordStatusText(record.status)} color={recordStatusColor(record.status)} />
        </div>
      </div>

      {/* 成绩复核状态与操作区（按角色展示申请 / 复核 / 回读） */}
      <section className="rounded-xl border border-amber-200 bg-amber-50 p-5">
        <div className="flex flex-wrap items-center justify-between gap-3">
          <div>
            <h2 className="font-semibold text-gray-800">成绩复核</h2>
            <p className="mt-1 text-xs text-gray-500">
              成绩发布后 24 小时内可发起一次复核；待复核期间成绩锁定，教师不能重复批改。
              {record.review_deadline && <span>（本成绩复核截止：{formatDateTime(record.review_deadline)}）</span>}
            </p>
          </div>
          {canApplyReview && (
            <button onClick={() => setApplyOpen(true)}
              className="rounded-lg bg-amber-600 px-4 py-2 text-sm text-white hover:bg-amber-700">
              申请复核
            </button>
          )}
          {windowExpired && (
            <span className="text-xs text-gray-400">复核窗口已关闭（发布超过 24 小时）</span>
          )}
          {review && (
            <StatusBadge text={reviewStatusText(review.status)} color={reviewStatusColor(review.status)} />
          )}
        </div>

        {/* 学生：申请理由回读 */}
        {review && isStudent && (
          <div className="mt-3 rounded-lg bg-white p-3 text-sm">
            <p><span className="text-gray-500">我的复核理由：</span>{review.reason}</p>
            <p className="mt-1 text-xs text-gray-400">申请时间 {formatDateTime(review.requested_at)}</p>
          </div>
        )}

        {/* 教师：待复核处理面板（仅可改主观题 + 复核意见必填） */}
        {canGrade && isPending && (
          <div className="mt-4 space-y-3 rounded-lg bg-white p-4">
            <p className="text-sm text-red-600">该成绩处于待复核状态，普通批改已锁定；请在复核中仅调整主观题分数。</p>
            <p className="text-sm"><span className="text-gray-500">学生复核理由：</span>{review?.reason}</p>
            <div className="grid gap-3 sm:grid-cols-2">
              {subjectiveQuestions.map((q) => (
                <div key={q.question_id} className="rounded-lg border border-gray-200 p-3">
                  <p className="truncate text-xs text-gray-500">{q.content}</p>
                  <p className="mt-1 text-xs text-gray-400">满分 {q.score} · 原得分 {q.subjective_score ?? 0}</p>
                  <div className="mt-2 grid grid-cols-2 gap-2">
                    <input type="number" min={0} max={q.score} step={0.5}
                      value={reviewScores[q.question_id] ?? ''}
                      placeholder="复核给分"
                      onChange={(e) => setReviewScores({ ...reviewScores, [q.question_id]: Number(e.target.value) })}
                      className="rounded-lg border border-gray-300 px-3 py-1.5 text-sm" />
                    <input value={reviewComments[q.question_id] ?? ''} placeholder="本题评语（可选）"
                      onChange={(e) => setReviewComments({ ...reviewComments, [q.question_id]: e.target.value })}
                      className="rounded-lg border border-gray-300 px-3 py-1.5 text-sm" />
                  </div>
                </div>
              ))}
            </div>
            <div>
              <label className="text-sm text-gray-600">复核意见 <span className="text-red-500">*</span></label>
              <textarea value={opinion} onChange={(e) => setOpinion(e.target.value)} rows={3} maxLength={500}
                placeholder="请填写复核意见（必填）"
                className="mt-1 w-full rounded-lg border border-gray-300 px-3 py-2 text-sm" />
            </div>
            <div className="flex justify-end">
              <button onClick={onCompleteReview} disabled={handling}
                className="rounded-lg bg-brand-600 px-6 py-2 text-sm text-white hover:bg-brand-700 disabled:opacity-60">
                {handling ? '提交中…' : '提交复核结果'}
              </button>
            </div>
          </div>
        )}

        {/* 复核完成：意见 + 修改前后分数 + 审计回读（所有角色可见，学生侧重结果，教师侧重审计） */}
        {review && review.status !== REVIEW_STATUS.PENDING && (
          <div className="mt-3 space-y-3 rounded-lg bg-white p-4 text-sm">
            {review.teacher_opinion && (
              <p><span className="text-gray-500">教师复核意见：</span>{review.teacher_opinion}</p>
            )}
            <div className="flex flex-wrap items-center gap-4 text-xs">
              <span>复核前总分 <b>{review.audit.before_final_score}</b>（客观 {review.audit.before_objective_score} / 主观 {review.audit.before_subjective_score}），
                {review.audit.before_passed ? '及格' : '不及格'}</span>
              <span>→</span>
              <span>复核后总分 <b className={review.audit.after_final_score !== review.audit.before_final_score ? 'text-amber-600' : ''}>{review.audit.after_final_score}</b>
                （客观 {review.audit.after_objective_score} / 主观 {review.audit.after_subjective_score}），
                <span className={review.audit.after_passed ? 'text-green-600' : 'text-red-600'}>{review.audit.after_passed ? '及格' : '不及格'}</span>
              </span>
            </div>
            {review.audit.score_changes && review.audit.score_changes.length > 0 && (
              <div className="text-xs">
                <p className="text-gray-500">分值变化明细：</p>
                <ul className="mt-1 list-disc pl-5">
                  {review.audit.score_changes.map((c) => (
                    <li key={c.question_id}>{c.type === 'fill' ? '填空题' : '简答题'} {c.question_id.slice(-4)}：{c.before} → {c.after} 分</li>
                  ))}
                </ul>
              </div>
            )}
            <p className="border-t border-gray-100 pt-2 text-xs text-gray-400">
              申请人 {review.audit.requested_by_name} · {formatDateTime(review.audit.requested_at)}
              {review.audit.handled_by_name && <> · 处理人 {review.audit.handled_by_name} · {formatDateTime(review.audit.handled_at)}</>}
              {canGrade && review.audit.handle_request_id && <> · request_id={review.audit.handle_request_id}</>}
            </p>
          </div>
        )}
      </section>

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
              {canGrade && (
                <div className="mt-3 grid gap-2 sm:grid-cols-2">
                  <input type="number" min={0} max={q.score} step={0.5} value={scores[q.question_id] ?? ''}
                    placeholder="给分" disabled={gradeLocked}
                    onChange={(e) => setScores({ ...scores, [q.question_id]: Number(e.target.value) })}
                    className="rounded-lg border border-gray-300 px-3 py-1.5 text-sm disabled:cursor-not-allowed disabled:bg-gray-100" />
                  <input value={comments[q.question_id] ?? ''} placeholder="评语（可选）" disabled={gradeLocked}
                    onChange={(e) => setComments({ ...comments, [q.question_id]: e.target.value })}
                    className="rounded-lg border border-gray-300 px-3 py-1.5 text-sm disabled:cursor-not-allowed disabled:bg-gray-100" />
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

      {/* 普通批改：非已批改完成态且未锁定时可保存；待复核期间隐藏（复核走上方专用面板） */}
      {canGrade && record.status !== RECORD_STATUS.GRADED && (
        <div className="flex items-center justify-between">
          {gradeLocked && <span className="text-xs text-red-500">成绩复核处理中，普通批改已锁定</span>}
          <button onClick={onGrade} disabled={grading || gradeLocked}
            className="ml-auto rounded-lg bg-brand-600 px-6 py-2 text-sm text-white hover:bg-brand-700 disabled:opacity-60">
            {grading ? '批改中…' : '保存批改'}
          </button>
        </div>
      )}
      {canGrade && record.status === RECORD_STATUS.GRADED && !isPending && (
        <p className="text-right text-xs text-gray-400">成绩已发布；如需修改可重新保存批改（学生复核窗口内可能发起复核）。</p>
      )}

      {/* 学生申请复核理由弹窗 */}
      <Modal open={applyOpen} title="申请成绩复核" onClose={() => setApplyOpen(false)}>
        <div className="space-y-3">
          <p className="text-xs text-gray-500">
            仅成绩发布后 24 小时内可申请，且每条成绩只能复核一次。提交后成绩将锁定，等待教师处理。
          </p>
          <textarea value={reason} onChange={(e) => setReason(e.target.value)} rows={4} maxLength={500}
            placeholder="请填写复核理由（必填）"
            className="w-full rounded-lg border border-gray-300 px-3 py-2 text-sm" />
          <div className="flex justify-end gap-2">
            <button onClick={() => setApplyOpen(false)}
              className="rounded-lg border border-gray-300 px-4 py-2 text-sm hover:bg-gray-50">取消</button>
            <button onClick={onApplyReview} disabled={submittingReview}
              className="rounded-lg bg-amber-600 px-4 py-2 text-sm text-white hover:bg-amber-700 disabled:opacity-60">
              {submittingReview ? '提交中…' : '确认提交'}
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
