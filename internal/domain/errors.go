package domain

import "errors"

var (
	ErrInvalid      = errors.New("输入不符合领域约束")
	ErrConflict     = errors.New("修订号冲突")
	ErrNotFound     = errors.New("对象不存在")
	ErrPublished    = errors.New("项目已发布，不允许继续写入")
	ErrInvalidState = errors.New("当前状态不允许此操作")
	ErrIdempotency  = errors.New("request_id 已用于不同请求")
	ErrApprovalGate = errors.New("项目尚未满足批准条件")
	ErrReviewer     = errors.New("批准人必须是指定复核员且不能是最终校样提交者")
)
