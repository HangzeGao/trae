// Package wal 实现本地 Write-Ahead Log，用于高风险操作的 fail-closed 审计（HA-07）。
//
// WAL 确保审计事件在业务事务提交前持久化到本地磁盘。
// 写入失败时返回错误，调用方应 fail-closed（回滚业务事务）。
//
// 文件名格式: wal-YYYYMMDD-HHMMSS-NNN.log
// 记录格式: seq(8) || timestamp(8) || event_len(4) || event_json
package wal

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"sync"
	"time"

	"github.com/HangzeGao/trae/key-vault/internal/config"
	domain "github.com/HangzeGao/trae/key-vault/internal/domain/audit"
)

// 记录头长度: seq(8) + timestamp(8) + event_len(4) = 20 字节
const recordHeaderLen = 20

// walFilePattern 匹配 WAL 文件名: wal-YYYYMMDD-HHMMSS-NNN.log
var walFilePattern = regexp.MustCompile(`^wal-\d{8}-\d{6}-\d{3}\.log$`)

// WAL 是本地追加写日志，用于高风险操作的 fail-closed 审计（HA-07）。
type WAL struct {
	dir         string
	maxSize     int64
	retention   time.Duration
	mu          sync.Mutex
	currentFD   *os.File
	currentSize int64
	seq         uint64
}

// New 创建 WAL 实例。
//
// 会创建目录（若不存在），并恢复上次的状态（打开最新文件、恢复序列号）。
// 若目录中无 WAL 文件，则创建新文件。
func New(cfg config.AuditConfig) (*WAL, error) {
	if cfg.WALDir == "" {
		return nil, fmt.Errorf("wal dir is empty")
	}

	// 创建目录，权限 0700 限制仅属主可访问
	if err := os.MkdirAll(cfg.WALDir, 0o700); err != nil {
		return nil, fmt.Errorf("create wal dir: %w", err)
	}

	maxSize := cfg.WALMaxSize
	if maxSize <= 0 {
		maxSize = 64 << 20 // 默认 64MiB
	}
	retention := cfg.WALRetention
	if retention <= 0 {
		retention = 7 * 24 * time.Hour // 默认 7 天
	}

	w := &WAL{
		dir:       cfg.WALDir,
		maxSize:   maxSize,
		retention: retention,
	}

	// 恢复上次状态：打开最新文件、恢复序列号
	if err := w.recover(); err != nil {
		return nil, fmt.Errorf("recover wal: %w", err)
	}

	// 若无当前文件，创建新文件
	if w.currentFD == nil {
		if err := w.createNewFileLocked(); err != nil {
			return nil, fmt.Errorf("create wal file: %w", err)
		}
	}

	return w, nil
}

// recover 扫描现有 WAL 文件，打开最新文件并恢复序列号。
//
// 通过顺序读取最新文件的记录来恢复最后写入的序列号，
// 确保序列号在重启后仍然单调递增。
func (w *WAL) recover() error {
	entries, err := os.ReadDir(w.dir)
	if err != nil {
		return fmt.Errorf("read wal dir: %w", err)
	}

	var files []string
	for _, e := range entries {
		if !e.IsDir() && walFilePattern.MatchString(e.Name()) {
			files = append(files, e.Name())
		}
	}

	if len(files) == 0 {
		return nil
	}

	// 按文件名排序，取最新文件
	sort.Strings(files)
	latest := files[len(files)-1]
	path := filepath.Join(w.dir, latest)

	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("stat wal file: %w", err)
	}

	// 以追加模式打开文件
	fd, err := os.OpenFile(path, os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return fmt.Errorf("open wal file: %w", err)
	}

	// 扫描文件以恢复最后序列号
	lastSeq, err := scanLastSeq(path)
	if err != nil {
		_ = fd.Close()
		return fmt.Errorf("scan last seq: %w", err)
	}

	w.currentFD = fd
	w.currentSize = info.Size()
	w.seq = lastSeq + 1

	return nil
}

// scanLastSeq 顺序读取 WAL 文件，返回最后一条记录的序列号。
//
// 文件格式: seq(8) || timestamp(8) || event_len(4) || event_json
// 若文件为空或损坏，返回 0。
func scanLastSeq(path string) (uint64, error) {
	f, err := os.Open(path)
	if err != nil {
		return 0, err
	}
	defer f.Close()

	header := make([]byte, recordHeaderLen)
	var lastSeq uint64

	for {
		_, err := io.ReadFull(f, header)
		if err == io.EOF || err == io.ErrUnexpectedEOF {
			// 文件末尾或部分写入（崩溃恢复场景），返回最后完整记录的序列号
			break
		}
		if err != nil {
			return 0, err
		}

		seq := binary.BigEndian.Uint64(header[0:8])
		eventLen := binary.BigEndian.Uint32(header[16:20])

		// 跳过事件 JSON 数据
		if eventLen > 0 {
			if _, err := io.CopyN(io.Discard, f, int64(eventLen)); err != nil {
				break
			}
		}

		lastSeq = seq
	}

	return lastSeq, nil
}

// createNewFileLocked 创建新的 WAL 文件（调用方需持锁）。
//
// 文件名格式: wal-YYYYMMDD-HHMMSS-NNN.log
// NNN 为同一秒内的序号，避免文件名冲突。
func (w *WAL) createNewFileLocked() error {
	now := time.Now()
	base := now.Format("20060102-150405")

	for i := 0; i < 1000; i++ {
		name := fmt.Sprintf("wal-%s-%03d.log", base, i)
		path := filepath.Join(w.dir, name)
		fd, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
		if err == nil {
			w.currentFD = fd
			w.currentSize = 0
			return nil
		}
		if !os.IsExist(err) {
			return err
		}
	}

	return fmt.Errorf("could not create unique wal file in %s", w.dir)
}

// Append 追加一条审计事件到 WAL。
//
// 格式: seq(8) || timestamp(8) || event_len(4) || event_json
// 写入后调用 fsync 确保持久化。
// 返回写入的序列号。
//
// 若当前文件超过 maxSize，会自动滚转。
// 写入失败时返回错误，调用方应 fail-closed（回滚业务事务）。
func (w *WAL) Append(ctx context.Context, event domain.Event) (uint64, error) {
	// 检查 context 是否已取消
	if err := ctx.Err(); err != nil {
		return 0, err
	}

	w.mu.Lock()
	defer w.mu.Unlock()

	// 若当前文件超过 maxSize，先滚转
	if w.currentSize >= w.maxSize {
		if err := w.rotateLocked(); err != nil {
			return 0, fmt.Errorf("rotate before append: %w", err)
		}
	}

	// 序列化事件为 JSON
	data, err := json.Marshal(event)
	if err != nil {
		return 0, fmt.Errorf("marshal event: %w", err)
	}

	// 构建记录: seq(8) || timestamp(8) || event_len(4) || event_json
	seq := w.seq
	w.seq++

	buf := make([]byte, recordHeaderLen+len(data))
	binary.BigEndian.PutUint64(buf[0:8], seq)
	binary.BigEndian.PutUint64(buf[8:16], uint64(event.Timestamp.UnixNano()))
	binary.BigEndian.PutUint32(buf[16:20], uint32(len(data)))
	copy(buf[20:], data)

	// 写入文件
	n, err := w.currentFD.Write(buf)
	if err != nil {
		return 0, fmt.Errorf("write wal: %w", err)
	}

	// fsync 确保持久化（HA-07 关键要求）
	if err := w.currentFD.Sync(); err != nil {
		return 0, fmt.Errorf("fsync wal: %w", err)
	}

	w.currentSize += int64(n)

	return seq, nil
}

// Rotate 滚转日志文件（当当前文件超过 maxSize 时）。
//
// 关闭当前文件并创建新文件。
func (w *WAL) Rotate() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.rotateLocked()
}

// rotateLocked 执行实际的滚转操作（调用方需持锁）。
func (w *WAL) rotateLocked() error {
	if w.currentFD != nil {
		if err := w.currentFD.Sync(); err != nil {
			return fmt.Errorf("fsync before rotate: %w", err)
		}
		if err := w.currentFD.Close(); err != nil {
			return fmt.Errorf("close current wal: %w", err)
		}
		w.currentFD = nil
	}
	return w.createNewFileLocked()
}

// Close 关闭 WAL，释放文件句柄。
func (w *WAL) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.currentFD != nil {
		err := w.currentFD.Close()
		w.currentFD = nil
		return err
	}
	return nil
}

// Cleanup 清理过期日志文件（超过 retention）。
//
// 不会删除当前正在写入的文件。
func (w *WAL) Cleanup() error {
	w.mu.Lock()
	defer w.mu.Unlock()

	entries, err := os.ReadDir(w.dir)
	if err != nil {
		return fmt.Errorf("read wal dir: %w", err)
	}

	cutoff := time.Now().Add(-w.retention)
	currentPath := ""
	if w.currentFD != nil {
		currentPath = w.currentFD.Name()
	}

	for _, e := range entries {
		if e.IsDir() || !walFilePattern.MatchString(e.Name()) {
			continue
		}

		info, err := e.Info()
		if err != nil {
			continue
		}

		// 跳过过期的文件
		if info.ModTime().After(cutoff) {
			continue
		}

		path := filepath.Join(w.dir, e.Name())
		// 不删除当前正在写入的文件
		if path == currentPath {
			continue
		}

		if err := os.Remove(path); err != nil {
			return fmt.Errorf("remove expired wal file %s: %w", e.Name(), err)
		}
	}

	return nil
}
