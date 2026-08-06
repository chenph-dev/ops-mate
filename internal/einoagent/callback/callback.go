// Package callback 提供智能体调用的观测日志（基于 eino callbacks）。
package callback

import (
	"context"
	"fmt"
	"log"

	"github.com/cloudwego/eino/callbacks"
	"github.com/cloudwego/eino/components/model"
)

// NewLogHandler 构造智能体调用日志 handler：
// 每次模型/工具调用结束打印一行（含 token 用量），失败打印错误。
// 通过 compose.WithCallbacks 挂到 Graph 节点，由组件内部触发。
func NewLogHandler() callbacks.Handler {
	builder := callbacks.NewHandlerBuilder()
	builder.OnEndFn(func(ctx context.Context, info *callbacks.RunInfo, output callbacks.CallbackOutput) context.Context {
		name := "组件"
		if info != nil && info.Name != "" {
			name = info.Name
		}
		var usage string
		if o, ok := output.(*model.CallbackOutput); ok && o.TokenUsage != nil {
			u := o.TokenUsage
			usage = logUsage(u.PromptTokens, u.CompletionTokens, u.TotalTokens)
		}
		log.Printf("einoagent: 调用 %s 完成%s", name, usage)
		return ctx
	})
	builder.OnErrorFn(func(ctx context.Context, info *callbacks.RunInfo, err error) context.Context {
		name := "组件"
		if info != nil && info.Name != "" {
			name = info.Name
		}
		log.Printf("einoagent: 调用 %s 失败: %v", name, err)
		return ctx
	})
	return builder.Build()
}

func logUsage(in, out, total int) string {
	// 仅有部分 provider 提供完整用量，缺失的字段以 0 展示
	return fmt.Sprintf(" tokens(in=%d out=%d total=%d)", in, out, total)
}
