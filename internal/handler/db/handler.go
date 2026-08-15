// Package db 提供数据库连接的 Wails 绑定 handler。
package db

import (
	"context"
	"fmt"
	"time"

	"ops-mate/internal/dbexec"
	"ops-mate/internal/handler/base"
)

// DbHandler 处理数据库查询与连接测试。
type DbHandler struct {
	resolver *base.ExecutorResolver
}

// NewDbHandler 构造 DbHandler。resolver 可为 nil（此时 ExecuteSQL 返回错误）。
func NewDbHandler(resolver *base.ExecutorResolver) *DbHandler {
	return &DbHandler{resolver: resolver}
}

// ExecuteSQL 在指定数据库资产上执行一条 SQL，返回结构化结果。
// 查询类（SELECT/SHOW 等）返回列与行；写类返回受影响行数。
func (h *DbHandler) ExecuteSQL(hostID, sqlText string) (*dbexec.Result, error) {
	ex := h.resolver.DbFor(hostID)
	if ex == nil {
		return nil, fmt.Errorf("无法解析数据库资产，请确认资产为数据库协议且凭据已录入")
	}
	ctx, cancel := context.WithTimeout(base.Ctx(), 30*time.Second)
	defer cancel()
	if dbexec.IsQuery(sqlText) {
		return ex.Query(ctx, sqlText)
	}
	return ex.Exec(ctx, sqlText)
}

// ListSchema 返回指定数据库资产的表/列结构（供前端左侧 schema 树）。
func (h *DbHandler) ListSchema(hostID string) (*dbexec.Schema, error) {
	ex := h.resolver.DbFor(hostID)
	if ex == nil {
		return nil, fmt.Errorf("无法解析数据库资产，请确认资产为数据库协议且凭据已录入")
	}
	ctx, cancel := context.WithTimeout(base.Ctx(), 30*time.Second)
	defer cancel()
	return ex.Schema(ctx)
}
