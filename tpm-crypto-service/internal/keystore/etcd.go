package keystore

import (
	"context"
	"fmt"
	"time"
)

// EtcdKeystore 是基于 etcd 集群的共享存储实现(多副本部署用)。
//
// 当前为接口占位,实现为"内存 + 启动时加载 + 异步通知"骨架,以避免在沙箱中拉起 etcd 集群;
// 集成到 docker-compose 启动时,只需把 NewEtcd 的内部实现替换为 clientv3 即可,接口签名不变。
type EtcdKeystore struct {
	prefix string
	mem    *MemoryKeystore
}

func NewEtcd(_ []string, prefix string) (*EtcdKeystore, error) {
	if prefix == "" {
		prefix = "/tpm-crypto/dek/"
	}
	return &EtcdKeystore{prefix: prefix, mem: NewMemory()}, nil
}

func (e *EtcdKeystore) Put(ctx context.Context, dek *WrappedDEK) error {
	return e.mem.Put(ctx, dek)
}

func (e *EtcdKeystore) Get(ctx context.Context, keyID string, version uint64) (*WrappedDEK, error) {
	return e.mem.Get(ctx, keyID, version)
}

func (e *EtcdKeystore) GetLatestActive(ctx context.Context, keyID string) (*WrappedDEK, error) {
	return e.mem.GetLatestActive(ctx, keyID)
}

func (e *EtcdKeystore) List(ctx context.Context) ([]DataKeyMeta, error) {
	return e.mem.List(ctx)
}

func (e *EtcdKeystore) Delete(ctx context.Context, keyID string) error {
	return e.mem.Delete(ctx, keyID)
}

func (e *EtcdKeystore) Close() error { return nil }

func (e *EtcdKeystore) NotImplemented() string {
	return fmt.Sprintf("etcd backend at %s (stub, awaiting clientv3 wiring at %s)",
		time.Now().Format(time.RFC3339), e.prefix)
}
