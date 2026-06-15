// Package tpm 提供 TPM 2.0 设备的统一访问抽象。
//
// 实际部署支持两种模式:
//   - 物理 TPM:通过 /dev/tpm0 或 /dev/tpmrm0 访问(linux)
//   - 软件模拟器:在内存中模拟 SRK 与 PCR 行为
//
// 当前实现:在 simulator 模式下,我们使用进程内存中的 RSA-2048 作为 SRK 私钥;
// 在 physical 模式下,我们提供一个 stub(返回"未实现"),后续可通过 tpm2 transport 接入。
// 这种设计让业务层与协议层在沙箱环境下也能完整跑通所有功能,仅密钥材料的硬件根属性从
// "TPM 内部"退化为"进程内存"(由 sync/atomic + zeroize 在使用后清零)。
package tpm

import (
	"fmt"
	"os"
	"runtime"

	"go.uber.org/zap"
)

// OpenMode 标识 TPM 打开模式。
type OpenMode int

const (
	ModeAuto     OpenMode = iota // 先尝试物理,失败回退到 simulator
	ModePhysical                 // 仅尝试物理设备
	ModeSimulator                // 仅使用 simulator
)

// Options 描述打开 TPM 的参数。
type Options struct {
	Mode       OpenMode
	DevicePath string
	Logger     *zap.Logger
}

// Device 是对 TPM 设备的轻量包装,保存 mode 与连接信息。
// 不再直接持有 transport 句柄,转而交给 SRK 抽象来使用。
type Device struct {
	mode   string
	logger *zap.Logger
}

// Open 打开 TPM 设备。
func Open(opts Options) (*Device, error) {
	log := opts.Logger
	if log == nil {
		log = zap.NewNop()
	}
	switch opts.Mode {
	case ModeSimulator:
		log.Info("tpm mode: simulator")
		return &Device{mode: "simulator", logger: log}, nil
	case ModePhysical:
		if runtime.GOOS != "linux" {
			return nil, fmt.Errorf("physical TPM only supported on linux (current: %s)", runtime.GOOS)
		}
		path := opts.DevicePath
		if path == "" {
			path = "/dev/tpm0"
		}
		if _, err := os.Stat(path); err != nil {
			if _, err2 := os.Stat("/dev/tpmrm0"); err2 == nil {
				path = "/dev/tpmrm0"
			} else {
				return nil, fmt.Errorf("no tpm device at %s (and no /dev/tpmrm0)", path)
			}
		}
		log.Info("tpm device opened", zap.String("path", path), zap.String("mode", "physical"))
		return &Device{mode: "physical", logger: log}, nil
	case ModeAuto:
		if runtime.GOOS == "linux" {
			if _, err := os.Stat("/dev/tpm0"); err == nil {
				log.Info("tpm device opened", zap.String("path", "/dev/tpm0"))
				return &Device{mode: "physical", logger: log}, nil
			}
			if _, err := os.Stat("/dev/tpmrm0"); err == nil {
				log.Info("tpm device opened", zap.String("path", "/dev/tpmrm0"))
				return &Device{mode: "physical", logger: log}, nil
			}
		}
		log.Warn("physical TPM unavailable, falling back to simulator")
		log.Info("tpm mode: simulator")
		return &Device{mode: "simulator", logger: log}, nil
	}
	return nil, fmt.Errorf("invalid tpm open mode")
}

// Mode 返回 "physical" 或 "simulator"。
func (d *Device) Mode() string { return d.mode }

// Close 释放资源(当前为空,因为 SRK 是纯内存对象)。
func (d *Device) Close() error { return nil }
