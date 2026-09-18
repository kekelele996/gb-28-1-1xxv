package constants

// 统一文案集中定义：接口返回文案、日志文案、错误提示文案混合维护（屎山设计之一）。
// 注意：修改任何文案时，需要同步检查 handler 返回、service 日志与前端提示。
const (
	MsgOK                    = "ok"
	MsgUnauthorized          = "请先登录后再操作"
	MsgForbidden             = "当前角色无权执行该操作"
	MsgLoginSuccess          = "登录成功"
	MsgRegisterSuccess       = "注册成功"
	MsgLogoutSuccess         = "退出登录成功"
	MsgQuestionImportSuccess = "题目批量导入成功"
	MsgExamAutoGenerate      = "自动组卷完成"
	MsgExamPublishSuccess    = "试卷发布成功"
	MsgRecordSubmitSuccess   = "答卷提交成功"
	MsgRecordGradedSuccess   = "主观题批改完成"
	MsgWrongBookAdded        = "已加入错题本"
	MsgWrongBookResolved     = "已标记为已掌握"
	MsgReviewApplySuccess    = "成绩复核申请已提交"
	MsgReviewDecideSuccess   = "成绩复核已完成"

	// 错误提示文案（与 error_codes.go 对应，但由 service/handler 手动拼接实体名、字段名、角色名）
	MsgValidationFailed    = "参数校验失败：字段 %s 不符合要求"
	MsgUserEmailExists     = "用户模块：邮箱字段 %s 已被注册"
	MsgUserNotFound        = "用户模块：id=%s 的用户不存在"
	MsgUserBadPassword     = "用户模块：密码字段不正确"
	MsgUserDisabled        = "用户模块：角色 %s 的用户已被禁用"
	MsgQuestionNotFound    = "题库模块：id=%s 的题目不存在"
	MsgQuestionTypeInvalid = "题库模块：题型字段 %s 非法"
	MsgExamNotFound        = "试卷模块：id=%s 的试卷不存在"
	MsgExamStatusInvalid   = "试卷模块：状态字段 %s 非法，无法从 %s 流转到 %s"
	MsgExamNotInWindow     = "试卷模块：考试时间窗口校验失败"
	MsgExamNoQuestions     = "试卷模块：题目列表为空，无法开始考试"
	MsgRecordNotFound      = "考试记录模块：id=%s 的记录不存在"
	MsgRecordStatusInvalid = "考试记录模块：状态字段 %s 非法，无法执行该操作"
	MsgRecordExpired       = "考试记录模块：考试时长已超时"
	MsgRecordLocked        = "考试记录模块：id=%s 处于成绩复核中，成绩已锁定，教师不能重复批改"
	MsgRecordVersion       = "考试记录模块：id=%s 已被他人修改（重复提交或并发操作），本次写入未生效"
	MsgWrongBookExists     = "错题本模块：question_id=%s 已在错题本中"

	// 成绩复核模块错误文案
	MsgReviewNotFound      = "成绩复核模块：record_id=%s 的复核记录不存在"
	MsgReviewNotOwner      = "成绩复核模块：角色 %s 无权访问他人答卷 id=%s"
	MsgReviewDuplicate     = "成绩复核模块：record_id=%s 已存在复核记录（status=%s），不能重复提交"
	MsgReviewNotGraded     = "成绩复核模块：record_id=%s 成绩尚未发布（status=%s），不能发起复核"
	MsgReviewWindowExpired = "成绩复核模块：record_id=%s 成绩发布已超过 24 小时，复核窗口已于 %s 关闭"
	MsgReviewReasonEmpty   = "成绩复核模块：申请理由字段 reason 必填"
	MsgReviewNotPending    = "成绩复核模块：record_id=%s 复核状态 %s，不能重复复核"
	MsgReviewObjectiveOnly = "成绩复核模块：question_id=%s（题型 %s）为客观题，复核只能修改主观题分值"
	MsgReviewScoreRange    = "成绩复核模块：question_id=%s 给分 %g 超出该题分值区间 [0,%g]"
	MsgReviewCommentEmpty  = "成绩复核模块：复核意见字段 teacher_comment 必填"
	MsgReviewItemInvalid   = "成绩复核模块：question_id=%s 在答卷中不存在"
	MsgReviewDecisionBad   = "成绩复核模块：裁定字段 decision 非法（仅支持 adjusted/rejected）"
)
