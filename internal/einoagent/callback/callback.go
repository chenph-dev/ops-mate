// Package callback 提供智能体调用的审计日志（基于 eino callbacks，落库 ai_call_logs）。
package callback

import (
	"context"
	"time"

	"github.com/cloudwego/eino/callbacks"
	"github.com/cloudwego/eino/components"
	"github.com/cloudwego/eino/components/model"

	logsstore "ops-mate/internal/store/logs"
)

// NewLogHandler 构造智能体调用审计 handler：
// 每次模型/工具调用结束落库一条记录（含 token 用量、耗时、成败）。
// sessionID 由调用方传入（每次 run 独立创建，避免共享实例无法区分会话）。
// 通过 compose.WithCallbacks 挂到 Graph 节点，由组件内部触发。
func NewLogHandler(logs *logsstore.LogsStore, sessionID string) callbacks.Handler {
	builder := callbacks.NewHandlerBuilder()
	builder.OnEndFn(func(ctx context.Context, info *callbacks.RunInfo, output callbacks.CallbackOutput) context.Context {
		log := logsstore.CallLog{
			SessionID: sessionID, Ts: time.Now().Unix(), OK: true,
			Component: componentOf(info), Name: nameOf(info), Provider: typeOf(info),
		}
		if o, ok := output.(*model.CallbackOutput); ok && o.TokenUsage != nil {
			log.TokensIn = o.TokenUsage.PromptTokens
			log.TokensOut = o.TokenUsage.CompletionTokens
			log.TokensTotal = o.TokenUsage.TotalTokens
		}
		if logs != nil {
			_ = logs.SaveLog(log)
		}
		return ctx
	})
	builder.OnErrorFn(func(ctx context.Context, info *callbacks.RunInfo, err error) context.Context {
		log := logsstore.CallLog{
			SessionID: sessionID, Ts: time.Now().Unix(), OK: false,
			Component: componentOf(info), Name: nameOf(info), Provider: typeOf(info),
			Error: err.Error(),
		}
		if logs != nil {
			_ = logs.SaveLog(log)
		}
		return ctx
	})
	return builder.Build()
}

func nameOf(info *callbacks.RunInfo) string {
	if info != nil && info.Name != "" {
		return info.Name
	}
	return "unknown"
}

func typeOf(info *callbacks.RunInfo) string {
	if info != nil {
		return info.Type
	}
	return ""
}

func componentOf(info *callbacks.RunInfo) string {
	if info != nil {
		// 归一为 "model" / "tool"（其余组件归为 "other"，实际主要是模型/工具节点）。
		switch info.Component {
		case components.ComponentOfChatModel:
			return "model"
		case components.ComponentOfTool:
			return "tool"
		}
	}
	return "other"
}
