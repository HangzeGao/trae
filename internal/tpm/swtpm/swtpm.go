// Package swtpm 提供 swtpm 进程管理能力。
//
// swtpm 是一个开源的软件 TPM 模拟器（https://github.com/stefanberger/swtpm），
// 在 P0 阶段用于集成测试，模拟真实 TPM2 硬件。本包负责拉起/停止 swtpm
// 进程，并通过 Unix socket 暴露控制接口。
package swtpm

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"sync"
	"syscall"
	"time"

	"github.com/HangzeGao/trae/key-vault/internal/observability"
)

// startTimeout 是 Start 等待 swtpm 进程就绪的最大时长。
const startTimeout = 5 * time.Second

// Manager 管理 swtpm 进程（P0 集成测试用）。
//
// 通过 exec.Command 拉起 swtpm 进程，监听 Unix socket；
// Stop 时发送 SIGTERM 优雅停止。
type Manager struct {
	socketPath string
	stateDir   string
	cmd        *exec.Cmd
	log        *observability.Logger
	mu         sync.Mutex
}

// NewManager 构造一个 swtpm 管理器。
//
// socketPath 是 swtpm 控制通道的 Unix socket 路径；
// stateDir 是 swtpm 持久化 TPM 状态的目录路径。
func NewManager(socketPath, stateDir string, log *observability.Logger) *Manager {
	return &Manager{
		socketPath: socketPath,
		stateDir:   stateDir,
		log:        log,
	}
}

// Start 启动 swtpm 进程。
//
// 命令格式：
//
//	swtpm socket --tpmstate dir=<stateDir> --ctrl type=unixio,path=<socketPath> --tpm2 --daemon
//
// 启动前会确保 stateDir 存在；若 swtpm 二进制不存在则返回错误。
func (m *Manager) Start(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.IsRunning() {
		return nil
	}

	// 确保 stateDir 存在。
	if err := os.MkdirAll(m.stateDir, 0o700); err != nil {
		return fmt.Errorf("swtpm: create state dir %q: %w", m.stateDir, err)
	}

	// 检查 swtpm 二进制是否可用。
	if _, err := exec.LookPath("swtpm"); err != nil {
		return fmt.Errorf("swtpm: binary not found in PATH: %w", err)
	}

	cmd := exec.CommandContext(ctx,
		"swtpm", "socket",
		"--tpmstate", fmt.Sprintf("dir=%s", m.stateDir),
		"--ctrl", fmt.Sprintf("type=unixio,path=%s", m.socketPath),
		"--tpm2",
		"--daemon",
	)
	if m.log != nil {
		cmd.Stdout = os.Stderr
		cmd.Stderr = os.Stderr
	}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("swtpm: start process: %w", err)
	}
	m.cmd = cmd

	// 等待 Unix socket 出现，判定 swtpm 就绪。
	if err := m.waitForSocket(startTimeout); err != nil {
		// 启动失败时尝试清理进程。
		_ = m.stopLocked()
		return err
	}

	if m.log != nil {
		m.log.Info("swtpm started",
			"socket", m.socketPath,
			"state_dir", m.stateDir,
			"pid", cmd.Process.Pid,
		)
	}
	return nil
}

// Stop 停止 swtpm 进程。
//
// 向 swtpm 进程发送 SIGTERM，等待其退出。若进程已不存在则视为成功。
func (m *Manager) Stop() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.stopLocked()
}

// stopLocked 是 Stop 的非加锁版本，调用方需持有 mu。
func (m *Manager) stopLocked() error {
	if m.cmd == nil || m.cmd.Process == nil {
		return nil
	}
	proc := m.cmd.Process
	pid := proc.Pid

	if err := proc.Signal(syscall.SIGTERM); err != nil && !errors.Is(err, os.ErrProcessDone) {
		return fmt.Errorf("swtpm: send SIGTERM: %w", err)
	}

	// 等待进程退出，超时后强制 kill。
	done := make(chan error, 1)
	go func() { done <- m.cmd.Wait() }()
	select {
	case <-done:
	case <-time.After(startTimeout):
		_ = proc.Kill()
		<-done
	}

	m.cmd = nil
	if m.log != nil {
		m.log.Info("swtpm stopped", "pid", pid)
	}
	return nil
}

// IsRunning 检查 swtpm 是否在运行。
//
// 通过检查 cmd 是否存在以及进程是否仍存活来判断。
func (m *Manager) IsRunning() bool {
	if m.cmd == nil || m.cmd.Process == nil {
		return false
	}
	// signal(0) 用于探测进程是否存活，不实际发送信号。
	if err := m.cmd.Process.Signal(syscall.Signal(0)); err != nil {
		return false
	}
	return true
}

// waitForSocket 阻塞等待 Unix socket 文件出现。
func (m *Manager) waitForSocket(timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for {
		if _, err := os.Stat(m.socketPath); err == nil {
			return nil
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("swtpm: socket %q not ready after %s", m.socketPath, timeout)
		}
		// 检查进程是否已退出。
		if m.cmd != nil && m.cmd.Process != nil {
			if err := m.cmd.Process.Signal(syscall.Signal(0)); err != nil {
				return fmt.Errorf("swtpm: process exited before socket ready: %w", err)
			}
		}
		time.Sleep(50 * time.Millisecond)
	}
}
