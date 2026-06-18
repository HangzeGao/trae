package nonce

import (
	"encoding/binary"
	stderrors "errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	kverr "github.com/HangzeGao/trae/key-vault/internal/errors"
)

// TestNonceToBytes 验证编码正确性，12 字节输出。
func TestNonceToBytes(t *testing.T) {
	got := NonceToBytes(0)
	if len(got) != NonceSize {
		t.Fatalf("输出长度应为 %d, got %d", NonceSize, len(got))
	}
	// 0 编码后高 4 字节为 0，低 8 字节为 0
	for i, b := range got {
		if b != 0 {
			t.Fatalf("NonceToBytes(0) 第 %d 字节应为 0, got %x", i, b)
		}
	}

	// 1 编码后低 8 字节大端为 1
	got = NonceToBytes(1)
	if binary.BigEndian.Uint64(got[4:]) != 1 {
		t.Fatalf("NonceToBytes(1) 低 8 字节应为 1, got %x", got[4:])
	}
	// 高 4 字节应为 0
	for i := 0; i < 4; i++ {
		if got[i] != 0 {
			t.Fatalf("NonceToBytes(1) 高 4 字节第 %d 位应为 0, got %x", i, got[i])
		}
	}
}

// TestNonceToBytesIncremental 验证递增 nonce 编码正确。
func TestNonceToBytesIncremental(t *testing.T) {
	prev := NonceToBytes(0)
	for i := uint64(1); i <= 100; i++ {
		cur := NonceToBytes(i)
		if len(cur) != NonceSize {
			t.Fatalf("输出长度应为 %d, got %d", NonceSize, len(cur))
		}
		// 解码后应等于 i
		got := binary.BigEndian.Uint64(cur[4:])
		if got != i {
			t.Fatalf("NonceToBytes(%d) 解码后 = %d, 不匹配", i, got)
		}
		// 与前一个不同
		if cur == prev {
			t.Fatalf("NonceToBytes(%d) 与前一个相同", i)
		}
		prev = cur
	}

	// 测试大数
	big := uint64(1) << 40
	got := NonceToBytes(big)
	if binary.BigEndian.Uint64(got[4:]) != big {
		t.Fatalf("NonceToBytes(%d) 解码不符", big)
	}
}

// TestLeaseManagerAllocate 验证从 lease 区间分配 nonce。
func TestLeaseManagerAllocate(t *testing.T) {
	var renewCount int32
	renew := func(kid string) (start, end uint64, expiresAt time.Time, err error) {
		atomic.AddInt32(&renewCount, 1)
		return 100, 199, time.Now().Add(time.Hour), nil
	}
	mgr := NewLeaseManager(renew, nil)

	// 首次分配应触发续租，返回区间起始 100
	n, err := mgr.Allocate("kid-1")
	if err != nil {
		t.Fatalf("首次 Allocate 失败: %v", err)
	}
	if n != 100 {
		t.Fatalf("首次 Allocate 应返回 100, got %d", n)
	}
	if atomic.LoadInt32(&renewCount) != 1 {
		t.Fatalf("应触发 1 次续租, got %d", renewCount)
	}

	// 后续分配应递增，不再触发续租
	for i := uint64(101); i < 150; i++ {
		n, err := mgr.Allocate("kid-1")
		if err != nil {
			t.Fatalf("Allocate 失败: %v", err)
		}
		if n != i {
			t.Fatalf("Allocate 应返回 %d, got %d", i, n)
		}
	}
	if atomic.LoadInt32(&renewCount) != 1 {
		t.Fatalf("区间未耗尽不应触发续租, got %d", renewCount)
	}

	// 不同 KID 应独立分配
	n2, err := mgr.Allocate("kid-2")
	if err != nil {
		t.Fatalf("Allocate(kid-2) 失败: %v", err)
	}
	if n2 != 100 {
		t.Fatalf("Allocate(kid-2) 应返回 100, got %d", n2)
	}
}

// TestLeaseManagerExhaustion 验证 lease 耗尽后续租失败返回错误。
func TestLeaseManagerExhaustion(t *testing.T) {
	// 续租函数返回错误，模拟后端不可用
	renewErr := stderrors.New("backend unavailable")
	renew := func(kid string) (start, end uint64, expiresAt time.Time, err error) {
		return 0, 0, time.Time{}, renewErr
	}
	mgr := NewLeaseManager(renew, nil)

	// 首次 Allocate 即续租失败，应返回 NonceExhausted
	_, err := mgr.Allocate("kid-exhausted")
	if err == nil {
		t.Fatal("续租失败应返回错误")
	}
	var ee *kverr.Error
	if !stderrors.As(err, &ee) {
		t.Fatalf("应返回 *errors.Error 类型, got %T", err)
	}
	if ee.Code != kverr.CodeNonceExhausted {
		t.Fatalf("错误码应为 NONCE_EXHAUSTED, got %s", ee.Code)
	}
}

// TestLeaseManagerExhaustionAfterRange 验证区间耗尽且续租失败时返回错误。
func TestLeaseManagerExhaustionAfterRange(t *testing.T) {
	// 第一次续租成功，返回 [0, 4]（5 个 nonce），之后续租失败
	var callCount int32
	renew := func(kid string) (start, end uint64, expiresAt time.Time, err error) {
		if atomic.AddInt32(&callCount, 1) == 1 {
			return 0, 4, time.Now().Add(time.Hour), nil
		}
		return 0, 0, time.Time{}, stderrors.New("exhausted")
	}
	mgr := NewLeaseManager(renew, nil)

	// 分配 5 个 nonce（区间 [0,4]）
	for i := uint64(0); i < 5; i++ {
		n, err := mgr.Allocate("kid-range")
		if err != nil {
			t.Fatalf("Allocate #%d 失败: %v", i, err)
		}
		if n != i {
			t.Fatalf("Allocate #%d 应返回 %d, got %d", i, i, n)
		}
	}

	// 第 6 次分配应触发续租且失败
	_, err := mgr.Allocate("kid-range")
	if err == nil {
		t.Fatal("区间耗尽且续租失败应返回错误")
	}
	var ee *kverr.Error
	if !stderrors.As(err, &ee) {
		t.Fatalf("应返回 *errors.Error 类型, got %T", err)
	}
	if ee.Code != kverr.CodeNonceExhausted {
		t.Fatalf("错误码应为 NONCE_EXHAUSTED, got %s", ee.Code)
	}
}

// TestLeaseManagerConcurrent 验证并发分配不重复。
func TestLeaseManagerConcurrent(t *testing.T) {
	// 区间 [0, 99999]，足够大避免耗尽
	renew := func(kid string) (start, end uint64, expiresAt time.Time, err error) {
		return 0, 99999, time.Now().Add(time.Hour), nil
	}
	mgr := NewLeaseManager(renew, nil)

	const goroutines = 50
	const allocsPerG = 100
	var wg sync.WaitGroup
	results := make([][]uint64, goroutines)
	for i := 0; i < goroutines; i++ {
		results[i] = make([]uint64, 0, allocsPerG)
	}

	wg.Add(goroutines)
	for i := 0; i < goroutines; i++ {
		go func(idx int) {
			defer wg.Done()
			for j := 0; j < allocsPerG; j++ {
				n, err := mgr.Allocate("kid-concurrent")
				if err != nil {
					t.Errorf("goroutine %d alloc %d 失败: %v", idx, j, err)
					return
				}
				results[idx] = append(results[idx], n)
			}
		}(i)
	}
	wg.Wait()

	// 汇总所有分配的 nonce，验证无重复
	seen := make(map[uint64]bool, goroutines*allocsPerG)
	total := 0
	for i := 0; i < goroutines; i++ {
		for _, n := range results[i] {
			total++
			if seen[n] {
				t.Fatalf("发现重复 nonce: %d", n)
			}
			seen[n] = true
		}
	}
	if total != goroutines*allocsPerG {
		t.Fatalf("总分配数应为 %d, got %d", goroutines*allocsPerG, total)
	}
}

// TestLeaseManagerExpiredRenewal 验证 lease 过期后自动续租。
func TestLeaseManagerExpiredRenewal(t *testing.T) {
	var renewCount int32
	renew := func(kid string) (start, end uint64, expiresAt time.Time, err error) {
		c := atomic.AddInt32(&renewCount, 1)
		if c == 1 {
			// 第一次返回立即过期的 lease
			return 0, 9, time.Now().Add(-time.Second), nil
		}
		// 第二次返回有效 lease
		return 100, 199, time.Now().Add(time.Hour), nil
	}
	mgr := NewLeaseManager(renew, nil)

	// 首次 Allocate：续租得到过期 lease，但因 needRenewLocked 已先续租，
	// 实际上第一次续租返回过期 lease 后仍会进入分配流程，
	// 但下一次 Allocate 会检测到过期并再次续租。
	n, err := mgr.Allocate("kid-expired")
	if err != nil {
		t.Fatalf("首次 Allocate 失败: %v", err)
	}
	if n != 0 {
		t.Fatalf("首次 Allocate 应返回 0, got %d", n)
	}

	// 第二次 Allocate：lease 已过期，应触发续租
	n, err = mgr.Allocate("kid-expired")
	if err != nil {
		t.Fatalf("第二次 Allocate 失败: %v", err)
	}
	if n != 100 {
		t.Fatalf("第二次 Allocate 应返回 100, got %d", n)
	}
	if atomic.LoadInt32(&renewCount) < 2 {
		t.Fatalf("应至少触发 2 次续租, got %d", renewCount)
	}
}

// TestLeaseManagerPrefetchCallback 验证使用量达 70% 时触发预取回调。
func TestLeaseManagerPrefetchCallback(t *testing.T) {
	var prefetchCount int32
	// 区间 [0, 9]，10 个 nonce，70% 阈值在第 7 次分配后触发
	renew := func(kid string) (start, end uint64, expiresAt time.Time, err error) {
		return 0, 9, time.Now().Add(time.Hour), nil
	}
	prefetch := func(kid string) {
		atomic.AddInt32(&prefetchCount, 1)
	}
	mgr := NewLeaseManager(renew, prefetch)

	// 分配 7 个 nonce（使用量 7/10 = 70%，应触发预取）
	for i := 0; i < 7; i++ {
		_, err := mgr.Allocate("kid-prefetch")
		if err != nil {
			t.Fatalf("Allocate #%d 失败: %v", i, err)
		}
	}
	// 等待异步预取回调执行
	time.Sleep(50 * time.Millisecond)
	if atomic.LoadInt32(&prefetchCount) < 1 {
		t.Fatalf("使用量达 70% 应触发预取回调, got %d", prefetchCount)
	}

	// 继续分配，预取回调不应重复触发（每个 lease 仅触发一次）
	prevCount := atomic.LoadInt32(&prefetchCount)
	for i := 0; i < 2; i++ {
		_, err := mgr.Allocate("kid-prefetch")
		if err != nil {
			t.Fatalf("Allocate #%d 失败: %v", 7+i, err)
		}
	}
	time.Sleep(50 * time.Millisecond)
	if atomic.LoadInt32(&prefetchCount) != prevCount {
		t.Fatalf("预取回调每个 lease 仅应触发一次, got %d (prev %d)", prefetchCount, prevCount)
	}
}

// TestLeaseManagerIsCritical 验证使用量达 90% 时进入 critical 水位。
func TestLeaseManagerIsCritical(t *testing.T) {
	// 区间 [0, 9]，10 个 nonce，90% 阈值在第 9 次分配后触发
	renew := func(kid string) (start, end uint64, expiresAt time.Time, err error) {
		return 0, 9, time.Now().Add(time.Hour), nil
	}
	mgr := NewLeaseManager(renew, nil)

	if mgr.IsCritical("kid-critical") {
		t.Fatal("未分配时不应处于 critical 水位")
	}

	// 分配 8 个 nonce（使用量 8/10 = 80%，未达 90%）
	for i := 0; i < 8; i++ {
		_, err := mgr.Allocate("kid-critical")
		if err != nil {
			t.Fatalf("Allocate #%d 失败: %v", i, err)
		}
	}
	if mgr.IsCritical("kid-critical") {
		t.Fatal("使用量 80% 不应处于 critical 水位")
	}

	// 第 9 次分配（使用量 9/10 = 90%），应进入 critical
	_, err := mgr.Allocate("kid-critical")
	if err != nil {
		t.Fatalf("Allocate #9 失败: %v", err)
	}
	if !mgr.IsCritical("kid-critical") {
		t.Fatal("使用量 90% 应处于 critical 水位")
	}
}

// TestSingleflightLeaseManager 验证 singleflight 包装器对相同 KID 的续租请求做合并。
func TestSingleflightLeaseManager(t *testing.T) {
	var renewCount int32
	var mu sync.Mutex
	renew := func(kid string) (start, end uint64, expiresAt time.Time, err error) {
		mu.Lock()
		defer mu.Unlock()
		atomic.AddInt32(&renewCount, 1)
		time.Sleep(20 * time.Millisecond) // 模拟慢速续租
		return 0, 999, time.Now().Add(time.Hour), nil
	}
	inner := NewLeaseManager(renew, nil)
	mgr := NewSingleflightLeaseManager(inner)

	// 并发对同一 KID 调用 Renew，应通过 singleflight 合并
	const n = 10
	var wg sync.WaitGroup
	wg.Add(n)
	for i := 0; i < n; i++ {
		go func() {
			defer wg.Done()
			_ = mgr.Renew("kid-sf")
		}()
	}
	wg.Wait()

	if atomic.LoadInt32(&renewCount) != 1 {
		t.Fatalf("singleflight 应合并为 1 次实际续租, got %d", renewCount)
	}

	// Allocate 应正常工作
	n2, err := mgr.Allocate("kid-sf")
	if err != nil {
		t.Fatalf("Allocate 失败: %v", err)
	}
	if n2 != 0 {
		t.Fatalf("Allocate 应返回 0, got %d", n2)
	}
}
