// Package keystore factory.
package keystore

import (
	"fmt"
)

// Open 根据 backend 字符串选择具体实现。
func Open(backend, path string, endpoints []string) (Keystore, error) {
	switch backend {
	case "memory", "":
		return NewMemory(), nil
	case "bolt":
		return NewBolt(path)
	case "etcd":
		return NewEtcd(endpoints, path)
	}
	return nil, fmt.Errorf("unknown keystore backend %q", backend)
}
