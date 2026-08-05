// Package model 提供 StreamingChatModel 流式包装层与 eino ChatModel 构造。
package model

import (
	"context"
	"errors"
	"io"
	"time"

	einomodel "github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
)

// modelTimeout 单次模型生成的超时。只覆盖模型调用（Stream），
// 防止 API 挂起导致会话永久卡在 Thinking；命令执行走 tools 节点独立 context，不受此限制。
// 若部分模型推理时间较长，可调大此值。
const modelTimeout = 120 * time.Second

// StreamingChatModel 把 eino ChatModel 包装为"生成即流式推送"的模型：
// Graph 以 Invoke 方式执行时，ChatModel 节点调用 Generate；
// 本包装层的 Generate 内部调用底层 Stream()，把每个 token 增量
// 以 ai:text 事件推给前端，同时累积完整消息返回给 Graph。
// 即"模型层流式、编排层同步"，规避 eino Graph 级流式与 Interrupt
// 组合的兼容性风险。
type StreamingChatModel struct {
	base        einomodel.ToolCallingChatModel
	sessionID   string
	emit        func(sessionID, event string, data any)
	onAssistant func(msg *schema.Message) // assistant 消息完成回调（用于落库）
}

// NewStreamingChatModel 构造包装层。emit/onAssistant 可为 nil（测试场景）。
func NewStreamingChatModel(
	base einomodel.ToolCallingChatModel,
	sessionID string,
	emit func(sessionID, event string, data any),
	onAssistant func(msg *schema.Message),
) *StreamingChatModel {
	return &StreamingChatModel{
		base: base, sessionID: sessionID,
		emit: emit, onAssistant: onAssistant,
	}
}

// Generate 内部走流式：逐块发 ai:text，累积后返回完整消息。
// 模型调用带独立超时（modelTimeout），超时后 Stream/Recv 返回错误，由上层转为错误事件，
// 会话回到 Idle 而非永久卡在 Thinking。
func (m *StreamingChatModel) Generate(ctx context.Context, input []*schema.Message, opts ...einomodel.Option) (*schema.Message, error) {
	modelCtx, cancel := context.WithTimeout(ctx, modelTimeout)
	defer cancel()
	sr, err := m.base.Stream(modelCtx, input, opts...)
	if err != nil {
		return nil, err
	}
	defer sr.Close()

	var chunks []*schema.Message
	for {
		chunk, err := sr.Recv()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, err
		}
		if chunk == nil {
			continue
		}
		if chunk.Content != "" && m.emit != nil {
			m.emit(m.sessionID, "ai:text", map[string]any{"delta": chunk.Content})
		}
		chunks = append(chunks, chunk)
	}

	if len(chunks) == 0 {
		return &schema.Message{Role: schema.Assistant}, nil
	}
	full, err := schema.ConcatMessages(chunks)
	if err != nil {
		return nil, err
	}
	// 串行约束：一次对话轮次只允许一条待审批命令。
	// 模型若一次返回多个 tool_call，只保留第一个，其余丢弃——
	// 否则 ToolsNode 会一次执行全部、每条都中断，而后端仅存单个 interruptID、
	// 前端仅有单个审批卡插槽，会导致"显示 pwd、实际执行 ls"的错配。
	if len(full.ToolCalls) > 1 {
		full.ToolCalls = full.ToolCalls[:1]
	}
	if m.onAssistant != nil {
		m.onAssistant(full)
	}
	return full, nil
}

// Stream 透传给底层模型。契约：调用方必须使用 Graph Invoke 模式（本包装层只在 Generate 中发事件/回调）；Stream 模式不会触发 emit 与 onAssistant。
func (m *StreamingChatModel) Stream(ctx context.Context, input []*schema.Message, opts ...einomodel.Option) (*schema.StreamReader[*schema.Message], error) {
	return m.base.Stream(ctx, input, opts...)
}

// WithTools 委托底层模型绑定工具，并返回包了一层的新实例。
func (m *StreamingChatModel) WithTools(tools []*schema.ToolInfo) (einomodel.ToolCallingChatModel, error) {
	bound, err := m.base.WithTools(tools)
	if err != nil {
		return nil, err
	}
	return NewStreamingChatModel(bound, m.sessionID, m.emit, m.onAssistant), nil
}
