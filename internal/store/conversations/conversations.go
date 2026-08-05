// Package convstore 管理对话、消息与命令记录的 CRUD。
package convstore

import (
	"fmt"
	"strings"
	"time"

	"ops-mate/internal/store"
	"ops-mate/internal/store/crypto"
)

// Conversation GORM 模型。
type convConversation struct {
	ID        string `gorm:"column:id;primaryKey"`
	HostID    string `gorm:"column:host_id"`
	Title     string `gorm:"column:title"`
	CreatedAt int64  `gorm:"column:created_at"`
	UpdatedAt int64  `gorm:"column:updated_at"`
}

func (convConversation) TableName() string { return "conversations" }

// Message GORM 模型。
type convMessage struct {
	ID             string  `gorm:"column:id;primaryKey"`
	SessionID      string  `gorm:"column:session_id"`
	Role           string  `gorm:"column:role"`
	Content        string  `gorm:"column:content"`
	ToolResult     *string `gorm:"column:tool_result"`
	ToolCalls      *string `gorm:"column:tool_calls"`
	ToolCallID     *string `gorm:"column:tool_call_id"`
	ToolName       *string `gorm:"column:tool_name"`
	ApprovalStatus *string `gorm:"column:approval_status"`
	Ts             int64   `gorm:"column:ts"`
}

func (convMessage) TableName() string { return "messages" }

// Command GORM 模型。
type convCommand struct {
	ID        string  `gorm:"column:id;primaryKey"`
	SessionID string  `gorm:"column:session_id"`
	Command   string  `gorm:"column:command"`
	ExitCode  *int    `gorm:"column:exit_code"`
	Output    *string `gorm:"column:output"`
	Ts        int64   `gorm:"column:ts"`
}

func (convCommand) TableName() string { return "commands" }

// Conversation 一次会话（DTO）。
type Conversation struct {
	ID        string `json:"id"`
	HostID    string `json:"hostId"`
	Title     string `json:"title"`
	CreatedAt int64  `json:"createdAt"`
	UpdatedAt int64  `json:"updatedAt"`
}

// Message 一条对话消息（DTO）。
type Message struct {
	ID             string `json:"id"`
	SessionID      string `json:"sessionId"`
	Role           string `json:"role"`
	Content        string `json:"content"`
	ToolResult     string `json:"toolResult"`
	ToolCalls      string `json:"toolCalls"`
	ToolCallID     string `json:"toolCallId"`
	ToolName       string `json:"toolName"`
	ApprovalStatus string `json:"approvalStatus"`
	Ts             int64  `json:"ts"`
}

// ConvStore 提供对话/消息/命令操作。
type ConvStore struct {
	app *store.DB
}

// NewConvStore 构造 ConvStore。
func NewConvStore(app *store.DB) *ConvStore {
	return &ConvStore{app: app}
}

func (s *ConvStore) NewConversation(hostID, title string) (string, error) {
	id := crypto.NewID()
	now := time.Now().Unix()
	err := s.app.GORM().Create(&convConversation{
		ID: id, HostID: hostID, Title: title,
		CreatedAt: now, UpdatedAt: now,
	}).Error
	if err != nil {
		return "", fmt.Errorf("insert conversation: %w", err)
	}
	return id, nil
}

func (s *ConvStore) ListConversations(hostID string) ([]Conversation, error) {
	var convs []convConversation
	if err := s.app.GORM().Where("host_id = ?", hostID).Order("updated_at desc").Find(&convs).Error; err != nil {
		return nil, err
	}
	out := make([]Conversation, 0, len(convs))
	for _, c := range convs {
		out = append(out, Conversation{
			ID: c.ID, HostID: c.HostID, Title: c.Title,
			CreatedAt: c.CreatedAt, UpdatedAt: c.UpdatedAt,
		})
	}
	return out, nil
}

// SaveMessage 落库一条消息（含 tool calling 字段），并刷新会话 updated_at。
// 会话首条 user 消息会用作标题摘要，让历史对话列表可读（而非"对话 <时间>"）。
func (s *ConvStore) SaveMessage(m Message) error {
	err := s.app.GORM().Create(&convMessage{
		ID: crypto.NewID(), SessionID: m.SessionID, Role: m.Role,
		Content: m.Content, ToolResult: strPtr(m.ToolResult),
		ToolCalls: strPtr(m.ToolCalls), ToolCallID: strPtr(m.ToolCallID),
		ToolName: strPtr(m.ToolName), ApprovalStatus: strPtr(m.ApprovalStatus),
		Ts: time.Now().Unix(),
	}).Error
	if err != nil {
		return err
	}
	if m.Role == "user" {
		var count int64
		if err := s.app.GORM().Model(&convMessage{}).
			Where("session_id = ?", m.SessionID).Count(&count).Error; err == nil && count == 1 {
			if sum := summarizeTitle(m.Content); sum != "" {
				_ = s.app.GORM().Model(&convConversation{}).
					Where("id = ?", m.SessionID).Update("title", sum).Error
			}
		}
	}
	return s.app.GORM().Model(&convConversation{}).
		Where("id = ?", m.SessionID).
		Update("updated_at", time.Now().Unix()).Error
}

// summarizeTitle 截取文本前 20 个字符作为会话标题摘要（rune 安全，中文不切碎）。
func summarizeTitle(text string) string {
	trimmed := strings.TrimSpace(text)
	runes := []rune(trimmed)
	if len(runes) <= 20 {
		return trimmed
	}
	return string(runes[:20]) + "..."
}

// GetConversation 按 ID 取会话。
func (s *ConvStore) GetConversation(id string) (Conversation, error) {
	var c convConversation
	if err := s.app.GORM().First(&c, "id = ?", id).Error; err != nil {
		return Conversation{}, err
	}
	return Conversation{
		ID: c.ID, HostID: c.HostID, Title: c.Title,
		CreatedAt: c.CreatedAt, UpdatedAt: c.UpdatedAt,
	}, nil
}

func (s *ConvStore) LoadMessages(sessionID string) ([]Message, error) {
	var msgs []convMessage
	// ts 是秒级：同秒多条消息时用 rowid（插入顺序）作为次级排序，
	// 保证 assistant(tool_calls) 与其 tool 结果的相对顺序稳定（审批状态推断依赖相邻关系）。
	if err := s.app.GORM().Where("session_id = ?", sessionID).Order("ts").Order("rowid").Find(&msgs).Error; err != nil {
		return nil, err
	}
	out := make([]Message, 0, len(msgs))
	for _, m := range msgs {
		out = append(out, Message{
			ID: m.ID, SessionID: m.SessionID, Role: m.Role,
			Content: m.Content, ToolResult: strDeref(m.ToolResult),
			ToolCalls: strDeref(m.ToolCalls), ToolCallID: strDeref(m.ToolCallID),
			ToolName: strDeref(m.ToolName), ApprovalStatus: strDeref(m.ApprovalStatus),
			Ts: m.Ts,
		})
	}
	return out, nil
}

func (s *ConvStore) SaveCommand(sessionID, command string, exitCode int, output string) error {
	return s.app.GORM().Create(&convCommand{
		ID: crypto.NewID(), SessionID: sessionID, Command: command,
		ExitCode: &exitCode, Output: strPtr(output),
		Ts: time.Now().Unix(),
	}).Error
}

func (s *ConvStore) DeleteConversation(id string) error {
	return s.app.GORM().Delete(&convConversation{}, "id = ?", id).Error
}

func strPtr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func strDeref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
