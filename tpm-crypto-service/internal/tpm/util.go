package tpm

import (
	"math/big"
)

// bytesToBigInt 是 small helper。
func bytesToBigInt(b []byte) *big.Int {
	return new(big.Int).SetBytes(b)
}
