// Package model 提供 StreamingChatModel 流式包装层与 eino ChatModel 构造。
package model

import (
	"context"
	"errors"
	"io"

	einomodel "github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
)

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
func (m *StreamingChatModel) Generate(ctx context.Context, input []*schema.Message, opts ...einomodel.Option) (*schema.Message, error) {
	sr, err := m.base.Stream(ctx, input, opts...)
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
