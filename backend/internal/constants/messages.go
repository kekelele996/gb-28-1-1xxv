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
	MsgReviewRequestSuccess  = "成绩复核申请已提交，待复核期间成绩锁定"
	MsgReviewHandleSuccess   = "成绩复核处理完成，总分与及格结果已更新"
	MsgWrongBookAdded        = "已加入错题本"
	MsgWrongBookResolved     = "已标记为已掌握"

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
	MsgWrongBookExists     = "错题本模块：question_id=%s 已在错题本中"

	// 成绩复核模块错误文案（实体名=成绩复核，字段名=reason/record_id/score，角色名=学生/教师）
	MsgReviewReasonRequired  = "成绩复核模块：字段 reason 必填，请填写复核理由"
	MsgReviewOpinionRequired = "成绩复核模块：字段 teacher_opinion 必填，请填写复核意见"
	MsgReviewNotGraded       = "成绩复核模块：record_id=%s 尚未批改发布成绩，无法发起复核"
	MsgReviewWindowClose     = "成绩复核模块：record_id=%s 的成绩发布已超 24 小时，复核窗口已关闭"
	MsgReviewDuplicate       = "成绩复核模块：record_id=%s 已存在复核记录（status=%s），每条成绩仅可复核一次"
	MsgReviewLocked          = "成绩复核模块：record_id=%s 处于待复核（pending）状态，成绩锁定，教师不能重复批改"
	MsgReviewNotFound        = "成绩复核模块：record_id=%s 不存在待处理复核"
	MsgReviewNotOwner        = "成绩复核模块：学生角色仅可对本人 record_id=%s 的成绩发起复核"
	MsgReviewObjectiveOnly   = "成绩复核模块：教师复核仅可调整主观题（fill/short），question_id=%s 为客观题不可改"
	MsgReviewScoreRange      = "成绩复核模块：question_id=%s 给分 %v 超出该题满分 %v"
	MsgReviewConflict        = "成绩复核模块：record_id=%s 原子落盘未命中（并发复核/重复提交/写入失败），记录、成绩与审计保持原样"
)
