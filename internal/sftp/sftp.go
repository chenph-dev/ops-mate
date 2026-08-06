// Package sftp 提供基于 SSH 的 SFTP 文件传输（目录浏览、上传、下载、删除、重命名、新建目录）。
// 上传/下载为可暂停/继续/取消的传输任务，支持进度回调。
package sftp

import (
	"context"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"sync"

	"github.com/pkg/sftp"

	"ops-mate/internal/sshexec"
)

// Entry 远端目录项。
type Entry struct {
	Name    string `json:"name"`
	IsDir   bool   `json:"isDir"`
	Size    int64  `json:"size"`
	Mode    string `json:"mode"`
	ModTime int64  `json:"modTime"`
}

// TaskStatus 传输任务状态。
type TaskStatus string

const (
	TaskQueued    TaskStatus = "queued"
	TaskRunning   TaskStatus = "running"
	TaskPaused    TaskStatus = "paused"
	TaskCancelled TaskStatus = "cancelled"
	TaskDone      TaskStatus = "done"
	TaskError     TaskStatus = "error"
)

// maxConcurrent 同时执行的传输任务数上限，其余进入队列等待。
const maxConcurrent = 2

// Task 一个上传/下载传输任务。
type Task struct {
	ID         string
	HostID     string
	Direction  string // "upload" | "download"
	LocalPath  string
	RemotePath string
	Total      int64
	Done       int64
	Status     TaskStatus
	ErrMsg     string

	mu   sync.Mutex
	cond *sync.Cond

	lastEmitted int64 // 进度事件节流（单 goroutine 访问）
}

// TaskInfo 任务对外快照（handler 返回前端）。
type TaskInfo struct {
	ID         string `json:"id"`
	Direction  string `json:"direction"`
	LocalPath  string `json:"localPath"`
	RemotePath string `json:"remotePath"`
	Total      int64  `json:"total"`
	Done       int64  `json:"done"`
	Status     string `json:"status"`
	Error      string `json:"error,omitempty"`
}

// progressEmit 传输进度回调（handler 注入，发 Wails 事件）。
type progressEmit func(t *Task)

// Manager 管理每主机的 SFTP 连接（懒建立、复用、随应用退出释放）与传输任务。
type Manager struct {
	mu       sync.Mutex
	clients  map[string]*sftp.Client
	tasks    map[string]*Task
	taskSeq  int
	hostFor  func(hostID string) (*sshexec.Host, error)
	progress progressEmit // 进度/终态
	start    progressEmit // 任务创建（开始）

	sem     chan struct{} // 并发槽信号量
	pending []*Task       // 排队等待启动的任务
}

// NewManager 构造 Manager。hostFor 按 hostID 解析主机凭据；progress/start 回调（可为 nil）。
func NewManager(
	hostFor func(hostID string) (*sshexec.Host, error),
	progress progressEmit,
	start progressEmit,
) *Manager {
	return &Manager{
		clients: map[string]*sftp.Client{}, tasks: map[string]*Task{},
		hostFor: hostFor, progress: progress, start: start,
		sem:     make(chan struct{}, maxConcurrent),
	}
}

// clientFor 获取（懒建立）主机的 SFTP 连接。
func (m *Manager) clientFor(ctx context.Context, hostID string) (*sftp.Client, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if c, ok := m.clients[hostID]; ok && c != nil {
		return c, nil
	}
	host, err := m.hostFor(hostID)
	if err != nil {
		return nil, fmt.Errorf("获取主机凭据: %w", err)
	}
	conn, err := sshexec.Dial(ctx, *host)
	if err != nil {
		return nil, err
	}
	client, err := sftp.NewClient(conn)
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("建立 SFTP: %w", err)
	}
	m.clients[hostID] = client
	return client, nil
}

// List 列目录，返回排序后的条目。
func (m *Manager) List(ctx context.Context, hostID, dir string) ([]Entry, error) {
	c, err := m.clientFor(ctx, hostID)
	if err != nil {
		return nil, err
	}
	infos, err := c.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("列目录 %s: %w", dir, err)
	}
	entries := make([]Entry, 0, len(infos))
	for _, fi := range infos {
		entries = append(entries, Entry{
			Name: fi.Name(), IsDir: fi.IsDir(), Size: fi.Size(),
			Mode: fi.Mode().String(), ModTime: fi.ModTime().Unix(),
		})
	}
	return entries, nil
}

// Mkdir 新建目录。
func (m *Manager) Mkdir(ctx context.Context, hostID, dir string) error {
	c, err := m.clientFor(ctx, hostID)
	if err != nil {
		return err
	}
	if err := c.Mkdir(dir); err != nil {
		return fmt.Errorf("新建目录 %s: %w", dir, err)
	}
	return nil
}

// Remove 删除文件或空目录。
func (m *Manager) Remove(ctx context.Context, hostID, p string) error {
	c, err := m.clientFor(ctx, hostID)
	if err != nil {
		return err
	}
	if err := c.Remove(p); err != nil {
		return fmt.Errorf("删除 %s: %w", p, err)
	}
	return nil
}

// Rename 重命名/移动。
func (m *Manager) Rename(ctx context.Context, hostID, oldPath, newPath string) error {
	c, err := m.clientFor(ctx, hostID)
	if err != nil {
		return err
	}
	if err := c.Rename(oldPath, newPath); err != nil {
		return fmt.Errorf("重命名 %s: %w", oldPath, err)
	}
	return nil
}

// ============ 传输任务 ============

// newTask 创建任务并登记。
func (m *Manager) newTask(hostID, direction, localPath, remotePath string, total int64) *Task {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.taskSeq++
	id := fmt.Sprintf("task-%d", m.taskSeq)
	t := &Task{
		ID: id, HostID: hostID, Direction: direction,
		LocalPath: localPath, RemotePath: remotePath,
		Total: total, Status: TaskRunning,
	}
	t.cond = sync.NewCond(&t.mu)
	m.tasks[id] = t
	// 任务开始事件：前端自动打开传输任务弹窗
	if m.start != nil {
		m.start(t)
	}
	return t
}

// StartUpload 启动上传任务：本地文件 → 远端目录，入队列等待执行。返回 taskID。
func (m *Manager) StartUpload(ctx context.Context, hostID, localPath, remoteDir string) (string, error) {
	stat, err := os.Stat(localPath)
	if err != nil {
		return "", fmt.Errorf("stat 本地文件: %w", err)
	}
	remotePath := path.Join(remoteDir, filepath.Base(localPath))
	t := m.newTask(hostID, "upload", localPath, remotePath, stat.Size())
	m.enqueue(t)
	return t.ID, nil
}

// StartDownload 启动下载任务：远端文件 → 本地路径，入队列等待执行。返回 taskID。
func (m *Manager) StartDownload(ctx context.Context, hostID, remotePath, localPath string) (string, error) {
	client, err := m.clientFor(ctx, hostID)
	if err != nil {
		return "", err
	}
	st, err := client.Stat(remotePath)
	if err != nil {
		return "", fmt.Errorf("stat 远端文件: %w", err)
	}
	t := m.newTask(hostID, "download", localPath, remotePath, st.Size())
	m.enqueue(t)
	return t.ID, nil
}

// enqueue 任务入队：标记 queued，加入等待队列，并尝试调度启动。
func (m *Manager) enqueue(t *Task) {
	t.mu.Lock()
	t.Status = TaskQueued
	t.mu.Unlock()
	m.mu.Lock()
	m.pending = append(m.pending, t)
	m.mu.Unlock()
	m.startNext()
}

// startNext 尽力调度：在有并发槽时逐个启动排队任务（跳过已取消的）。
func (m *Manager) startNext() {
	for {
		m.mu.Lock()
		if len(m.pending) == 0 {
			m.mu.Unlock()
			return
		}
		select {
		case m.sem <- struct{}{}:
			t := m.pending[0]
			m.pending = m.pending[1:]
			// t.Status 受 t.mu 保护（CancelTask 并发写）
			t.mu.Lock()
			if t.Status == TaskCancelled {
				t.mu.Unlock()
				<-m.sem // 已被取消，释放槽继续下一个
				m.mu.Unlock()
				continue
			}
			t.Status = TaskRunning
			t.mu.Unlock()
			m.mu.Unlock()
			go m.runTask(t)
		default:
			m.mu.Unlock()
			return
		}
	}
}

// runTask 执行任务并在结束时释放并发槽、调度下一个。
func (m *Manager) runTask(t *Task) {
	defer func() {
		<-m.sem
		m.startNext()
	}()
	switch t.Direction {
	case "upload":
		m.runUpload(t)
	case "download":
		m.runDownload(t)
	}
}

// PauseTask 暂停任务。
func (m *Manager) PauseTask(id string) error {
	t := m.getTask(id)
	if t == nil {
		return fmt.Errorf("任务不存在")
	}
	t.mu.Lock()
	if t.Status == TaskRunning {
		t.Status = TaskPaused
	}
	t.mu.Unlock()
	return nil
}

// ResumeTask 继续任务。
func (m *Manager) ResumeTask(id string) error {
	t := m.getTask(id)
	if t == nil {
		return fmt.Errorf("任务不存在")
	}
	t.mu.Lock()
	if t.Status == TaskPaused {
		t.Status = TaskRunning
		t.cond.Signal()
	}
	t.mu.Unlock()
	return nil
}

// CancelTask 取消任务（保留记录，状态置 cancelled；进行中的后台 goroutine 检测后退出，
// 队列中的任务由 startNext 跳过）。前端「已删除」页签查看，可再用 RemoveTask 移除记录。
func (m *Manager) CancelTask(id string) error {
	m.mu.Lock()
	t, ok := m.tasks[id]
	m.mu.Unlock()
	if !ok {
		return fmt.Errorf("任务不存在")
	}
	t.mu.Lock()
	if t.Status == TaskRunning || t.Status == TaskPaused || t.Status == TaskQueued {
		t.Status = TaskCancelled
		t.cond.Signal()
	}
	t.mu.Unlock()
	return nil
}

// RemoveTask 从记录中移除任务（清理已结束/已取消的任务）。
func (m *Manager) RemoveTask(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.tasks[id]; !ok {
		return fmt.Errorf("任务不存在")
	}
	delete(m.tasks, id)
	return nil
}

// ListTasks 返回任务快照。
func (m *Manager) ListTasks() []TaskInfo {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]TaskInfo, 0, len(m.tasks))
	for _, t := range m.tasks {
		t.mu.Lock()
		out = append(out, TaskInfo{
			ID: t.ID, Direction: t.Direction,
			LocalPath: t.LocalPath, RemotePath: t.RemotePath,
			Total: t.Total, Done: t.Done,
			Status: string(t.Status), Error: t.ErrMsg,
		})
		t.mu.Unlock()
	}
	return out
}

func (m *Manager) getTask(id string) *Task {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.tasks[id]
}

// runUpload 上传任务主体。
func (m *Manager) runUpload(t *Task) {
	client, err := m.clientFor(context.Background(), t.HostID)
	if err != nil {
		t.finish(TaskError, err.Error())
		return
	}
	local, err := os.Open(t.LocalPath)
	if err != nil {
		t.finish(TaskError, err.Error())
		return
	}
	defer local.Close()
	remote, err := client.Create(t.RemotePath)
	if err != nil {
		t.finish(TaskError, err.Error())
		return
	}
	defer remote.Close()
	m.copyLoop(t, local, remote)
}

// runDownload 下载任务主体。
func (m *Manager) runDownload(t *Task) {
	client, err := m.clientFor(context.Background(), t.HostID)
	if err != nil {
		t.finish(TaskError, err.Error())
		return
	}
	remote, err := client.Open(t.RemotePath)
	if err != nil {
		t.finish(TaskError, err.Error())
		return
	}
	defer remote.Close()
	local, err := os.Create(t.LocalPath)
	if err != nil {
		t.finish(TaskError, err.Error())
		return
	}
	defer local.Close()
	m.copyLoop(t, remote, local)
}

// copyLoop 块级复制循环：支持暂停（cond.Wait）、取消、进度回调与节流。
func (m *Manager) copyLoop(t *Task, r io.Reader, w io.Writer) {
	buf := make([]byte, 32*1024)
	for {
		t.mu.Lock()
		for t.Status == TaskPaused {
			t.cond.Wait()
		}
		if t.Status == TaskCancelled {
			t.mu.Unlock()
			return
		}
		t.mu.Unlock()

		n, rerr := r.Read(buf)
		if n > 0 {
			if _, werr := w.Write(buf[:n]); werr != nil {
				t.finish(TaskError, werr.Error())
				m.emitFinal(t)
				return
			}
			t.mu.Lock()
			t.Done += int64(n)
			status := t.Status
			t.mu.Unlock()
			// 进度节流：每 512KB 或状态变化时回调
			if m.progress != nil && (t.Done-t.lastEmitted >= 512*1024 || status != TaskRunning) {
				t.lastEmitted = t.Done
				m.progress(t)
			}
			if status == TaskCancelled {
				return
			}
		}
		if rerr == io.EOF {
			t.finish(TaskDone, "")
			m.emitFinal(t)
			return
		}
		if rerr != nil {
			t.finish(TaskError, rerr.Error())
			m.emitFinal(t)
			return
		}
	}
}

// emitFinal 发送最终状态（绕过节流）——任务完成/失败后必须调用，
// 否则前端收不到 done/error 状态，进度到 100% 但状态标签与按钮不更新。
func (m *Manager) emitFinal(t *Task) {
	if m.progress != nil {
		m.progress(t)
	}
}

// finish 结束任务（取消优先级最高）。
func (t *Task) finish(s TaskStatus, errMsg string) {
	t.mu.Lock()
	if t.Status != TaskCancelled {
		t.Status = s
		t.ErrMsg = errMsg
	}
	t.mu.Unlock()
}

// Close 关闭全部连接与任务（应用退出时调用）。
func (m *Manager) Close() {
	m.mu.Lock()
	defer m.mu.Unlock()
	for id, c := range m.clients {
		_ = c.Close()
		delete(m.clients, id)
	}
	m.tasks = map[string]*Task{}
}
