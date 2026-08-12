// Package termctx 提供终端输出的环形缓存与清洗压缩，供 AI 上下文注入使用。
package termctx

import "sync"

// RingBuffer 按 hostID 缓存最近终端原始输出（字节），超限丢弃最旧。
type RingBuffer struct {
	mu   sync.Mutex
	data []byte
	max  int
}

// NewRingBuffer 构造环形缓冲区。max<=0 表示不缓存任何内容。
func NewRingBuffer(max int) *RingBuffer {
	return &RingBuffer{max: max}
}

// Append 追加一段输出；超出 max 时从头部丢弃最旧内容。
func (b *RingBuffer) Append(p []byte) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.max <= 0 || len(p) == 0 {
		return
	}
	b.data = append(b.data, p...)
	if len(b.data) > b.max {
		b.data = append([]byte(nil), b.data[len(b.data)-b.max:]...)
	}
}

// Reset 清空缓冲区（终端清屏时调用）。
func (b *RingBuffer) Reset() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.data = nil
}

// Bytes 返回缓冲内容副本（线程安全）。
func (b *RingBuffer) Bytes() []byte {
	b.mu.Lock()
	defer b.mu.Unlock()
	return append([]byte(nil), b.data...)
}
