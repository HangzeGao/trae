package keyresolver

import (
	"context"
)

// prefetchRequest 是异步预取请求，由 PrefetchDEK 发送到 channel，
// 由 StartPrefetchWorker 启动的后台 worker 消费处理。
type prefetchRequest struct {
	TenantID string
	KeyID    string
	Version  int
}

// PrefetchDEK 预取 DEK 到缓存。
//
// 密钥创建/轮转后，管理面调用此方法预热数据面缓存，使后续数据面请求能命中缓存。
// 此方法为非阻塞：将预取请求发送到 channel，由后台 worker 异步处理。
// 若 channel 已满则丢弃请求（预取为 best-effort，不影响正确性，下次请求会正常解封）。
func (r *Resolver) PrefetchDEK(ctx context.Context, tenantID, keyID string, version int) error {
	req := prefetchRequest{
		TenantID: tenantID,
		KeyID:    keyID,
		Version:  version,
	}
	select {
	case r.prefetchCh <- req:
		return nil
	default:
		// channel 已满，丢弃请求（best-effort 预取）
		r.log.Warn("prefetch channel full, dropping request",
			"key_id", keyID,
			"version", version,
		)
		return nil
	}
}

// StartPrefetchWorker 启动后台预取 worker。
//
// 监听预取请求 channel，异步处理。worker 在以下情况退出：
//   - resolver 内部 context 取消（Close 调用）
//   - 调用方传入的 ctx 取消
//
// 应在服务启动时调用此方法。worker 内部调用 IssueDEKLease 逻辑预热缓存，
// 复用 singleflight 合并机制，避免与数据面请求重复解封。
func (r *Resolver) StartPrefetchWorker(ctx context.Context) {
	r.wg.Add(1)
	go func() {
		defer r.wg.Done()
		for {
			select {
			case <-r.ctx.Done():
				return
			case <-ctx.Done():
				return
			case req := <-r.prefetchCh:
				r.processPrefetch(req)
			}
		}
	}()
}

// processPrefetch 处理单个预取请求。
//
// 若 DEK 已缓存则跳过；否则调用 IssueDEKLease 触发解封并缓存。
// 预取失败仅记录日志，不影响数据面正确性。
func (r *Resolver) processPrefetch(req prefetchRequest) {
	kid := formatKID(req.KeyID, req.Version)

	// 若已缓存则跳过
	if _, ok := r.dekCache.Get(kid); ok {
		return
	}

	// 使用 resolver 生命周期 context 派生超时 context
	prefetchCtx, cancel := context.WithTimeout(r.ctx, r.config.ResolverTimeout)
	defer cancel()

	// 调用 IssueDEKLease 触发解封并缓存
	// 复用 singleflight 机制，避免与数据面请求重复解封
	_, err := r.IssueDEKLease(prefetchCtx, req.TenantID, req.KeyID, req.Version)
	if err != nil {
		r.log.Warn("prefetch DEK failed",
			"error", err,
			"key_id", req.KeyID,
			"version", req.Version,
		)
	}
}
