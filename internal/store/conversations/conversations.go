// Package convstore 管理对话、消息与命令记录的 CRUD。
package convstore

import (
	"fmt"
	"time"

	"ops-mate/internal/store"
	"ops-mate/internal/store/crypto"
)

// Conversation 一次会话。
type Conversation struct {
	ID        string `json:"id"`
	HostID    string `json:"hostId"`
	Title     string `json:"title"`
	CreatedAt int64  `json:"createdAt"`
	UpdatedAt int64  `json:"updatedAt"`
}

// Message 一条对话消息。
type Message struct {
	ID         string `json:"id"`
	SessionID  string `json:"sessionId"`
	Role       string `json:"role"`
	Content    string `json:"content"`
	ToolResult string `json:"toolResult"`
	Ts         int64  `json:"ts"`
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
	_, err := s.app.DB().Exec(
		`INSERT INTO conversations(id,host_id,title,created_at,updated_at) VALUES(?,?,?,?,?)`,
		id, hostID, title, now, now)
	if err != nil {
		return "", fmt.Errorf("insert conversation: %w", err)
	}
	return id, nil
}

func (s *ConvStore) ListConversations(hostID string) ([]Conversation, error) {
	rows, err := s.app.DB().Query(
		`SELECT id,host_id,title,created_at,updated_at FROM conversations WHERE host_id=? ORDER BY updated_at DESC`,
		hostID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Conversation
	for rows.Next() {
		var c Conversation
		if err := rows.Scan(&c.ID, &c.HostID, &c.Title, &c.CreatedAt, &c.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func (s *ConvStore) AppendMessage(sessionID, role, content, toolResult string) error {
	id := crypto.NewID()
	_, err := s.app.DB().Exec(
		`INSERT INTO messages(id,session_id,role,content,tool_result,ts) VALUES(?,?,?,?,?,?)`,
		id, sessionID, role, content, crypto.Nullable(toolResult), time.Now().Unix())
	if err != nil {
		return err
	}
	_, err = s.app.DB().Exec(`UPDATE conversations SET updated_at=? WHERE id=?`, time.Now().Unix(), sessionID)
	return err
}

func (s *ConvStore) LoadMessages(sessionID string) ([]Message, error) {
	rows, err := s.app.DB().Query(
		`SELECT id,session_id,role,content,tool_result,ts FROM messages WHERE session_id=? ORDER BY ts`,
		sessionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Message
	for rows.Next() {
		var m Message
		var tr *string
		if err := rows.Scan(&m.ID, &m.SessionID, &m.Role, &m.Content, &tr, &m.Ts); err != nil {
			return nil, err
		}
		if tr != nil {
			m.ToolResult = *tr
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

func (s *ConvStore) SaveCommand(sessionID, command string, exitCode int, output string) error {
	id := crypto.NewID()
	_, err := s.app.DB().Exec(
		`INSERT INTO commands(id,session_id,command,exit_code,output,ts) VALUES(?,?,?,?,?,?)`,
		id, sessionID, command, exitCode, crypto.Nullable(output), time.Now().Unix())
	return err
}

func (s *ConvStore) DeleteConversation(id string) error {
	_, err := s.app.DB().Exec(`DELETE FROM conversations WHERE id=?`, id)
	return err
}
