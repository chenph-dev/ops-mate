// Package base 提供 handler 层共享的基础设施：Wails 应用上下文与主机连接解析。
package base

import "context"

// ctx 是 Wails 应用在 OnStartup 时注入的上下文，供各 handler 子包内部使用。
// 作为包级变量而非 handler 字段，避免 Wails 为 SetCtx 方法生成
// context.Context 的 models 类型（Wails 无法为接口生成模型，会导致悬空 import）。
var ctx context.Context

// SetCtx 由 main.go 的 OnStartup 调用，注入 Wails 上下文。
// 包级函数而非方法，Wails 不会为其生成前端绑定。
func SetCtx(c context.Context) {
	ctx = c
}

// Ctx 返回当前 Wails 上下文，供 handler 内部使用。
func Ctx() context.Context {
	return ctx
}
