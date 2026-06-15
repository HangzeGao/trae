package tpm

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/gob"
	"errors"
	"fmt"
	"os"
)

// SRK 是 Storage Root Key 的逻辑抽象;在硬件 TPM 模式下其私钥永不出 TPM;
// 在 simulator 模式下它退化为进程内存中的 RSA-2048 密钥,通过 state 文件持久化
// 以保证重启后能恢复同一把密钥(模拟"TPM 内部不可变"的语义)。
//
// 对业务层而言,SRK 只暴露 Seal/Unseal 两个方法,无须关心底层的 transport 细节。
type SRK struct {
	// Mode 标识当前为 "physical" 或 "simulator"
	Mode string
	// pubKey 用于 Seal 时的公钥
	pubKey rsa.PublicKey
	// privKey 仅 simulator 模式下非 nil
	privKey *rsa.PrivateKey
	// Handle 描述句柄
	Handle string
	// statePath 持久化私钥的路径(simulator 模式专用)
	statePath string
}

// SealedARK 是 ARK 的密文形态,落盘存储。
// 设计:ARK(32B 随机)经 SRK 公钥 RSA-OAEP 加密得到 ciphertext;
// 同时记录封存时刻的 PCR 摘要,Unseal 时校验当前 PCR 是否匹配(fail-closed)。
type SealedARK struct {
	Version   uint8  // 格式版本
	Algorithm string // "RSA-OAEP-SHA256"
	PCRDigest []byte // 封存时刻的 PCR 摘要
	PCRs      []int  // 参与摘要的 PCR 索引
	Cipher    []byte // RSA-OAEP(ARK)
}

const sealedARKVersion uint8 = 1

// EnsureSRK 创建或获取 SRK。
//  - mode="simulator":尝试从 statePath 加载,否则生成并保存。
//  - mode="physical":未实现,返回错误。
func EnsureSRK(mode, statePath string) (*SRK, error) {
	if mode == "simulator" {
		if statePath == "" {
			return nil, fmt.Errorf("simulator mode requires statePath")
		}
		// 尝试加载
		if s, err := loadSRKState(statePath); err == nil {
			return s, nil
		}
		// 生成
		priv, err := rsa.GenerateKey(rand.Reader, 2048)
		if err != nil {
			return nil, fmt.Errorf("generate SRK: %w", err)
		}
		s := &SRK{
			Mode:      "simulator",
			pubKey:    priv.PublicKey,
			privKey:   priv,
			Handle:    "simulator://" + statePath,
			statePath: statePath,
		}
		if err := saveSRKState(statePath, s); err != nil {
			return nil, fmt.Errorf("save SRK state: %w", err)
		}
		return s, nil
	}
	return nil, fmt.Errorf("physical SRK must be created via tpm2 transport (not implemented in this build)")
}

type srkState struct {
	D  []byte
	N  []byte
	E  int
}

func saveSRKState(path string, s *SRK) error {
	st := srkState{
		D: s.privKey.D.Bytes(),
		N: s.privKey.N.Bytes(),
		E: s.privKey.E,
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return err
	}
	defer f.Close()
	return gob.NewEncoder(f).Encode(st)
}

func loadSRKState(path string) (*SRK, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var st srkState
	if err := gob.NewDecoder(f).Decode(&st); err != nil {
		return nil, err
	}
	priv := &rsa.PrivateKey{
		PublicKey: rsa.PublicKey{
			N: bytesToBigInt(st.N),
			E: st.E,
		},
		D: bytesToBigInt(st.D),
	}
	return &SRK{
		Mode:      "simulator",
		pubKey:    priv.PublicKey,
		privKey:   priv,
		Handle:    "simulator://" + path,
		statePath: path,
	}, nil
}

// SealARK 封存 32 字节的 ARK 明文。
func (s *SRK) SealARK(ark []byte, pcrs []int) (*SealedARK, error) {
	if len(ark) != 32 {
		return nil, fmt.Errorf("ARK must be 32 bytes, got %d", len(ark))
	}
	digest, err := currentPCRDigest(pcrs)
	if err != nil {
		return nil, fmt.Errorf("read PCR digest: %w", err)
	}
	ct, err := rsa.EncryptOAEP(sha256.New(), rand.Reader, &s.pubKey, ark, digest)
	if err != nil {
		return nil, fmt.Errorf("RSA-OAEP encrypt: %w", err)
	}
	return &SealedARK{
		Version:   sealedARKVersion,
		Algorithm: "RSA-OAEP-SHA256",
		PCRDigest: digest,
		PCRs:      pcrs,
		Cipher:    ct,
	}, nil
}

// UnsealARK 还原 ARK 明文;PCR 摘要必须与封存时一致。
func (s *SRK) UnsealARK(sealed *SealedARK) ([]byte, error) {
	if sealed == nil {
		return nil, errors.New("nil SealedARK")
	}
	if sealed.Version != sealedARKVersion {
		return nil, fmt.Errorf("unsupported SealedARK version %d", sealed.Version)
	}
	cur, err := currentPCRDigest(sealed.PCRs)
	if err != nil {
		return nil, fmt.Errorf("read current PCR: %w", err)
	}
	if !bytesEqual(cur, sealed.PCRDigest) {
		return nil, fmt.Errorf("PCR digest mismatch: fail-closed")
	}
	if s.privKey == nil {
		return nil, fmt.Errorf("SRK private key unavailable (hardware path not wired in this build)")
	}
	pt, err := rsa.DecryptOAEP(sha256.New(), rand.Reader, s.privKey, sealed.Cipher, sealed.PCRDigest)
	if err != nil {
		return nil, fmt.Errorf("RSA-OAEP decrypt: %w", err)
	}
	if len(pt) != 32 {
		return nil, fmt.Errorf("unexpected ARK length %d", len(pt))
	}
	return pt, nil
}

// currentPCRDigest 提供一个软件占位的 PCR 摘要:对传入的 pcrs 索引排序后,拼接索引和进程随机数;
// 在真实硬件上,TPM2_PCR_Read 会给出固定摘要;这里为模拟器提供一个稳定但随机的 32B 摘要。
func currentPCRDigest(pcrs []int) ([]byte, error) {
	sorted := append([]int(nil), pcrs...)
	for i := 1; i < len(sorted); i++ {
		for j := i; j > 0 && sorted[j-1] > sorted[j]; j-- {
			sorted[j-1], sorted[j] = sorted[j], sorted[j-1]
		}
	}
	h := sha256.New()
	for _, p := range sorted {
		fmt.Fprintf(h, "PCR%d", p)
	}
	h.Write(stablePCRSeed())
	return h.Sum(nil), nil
}

var pcrSeed []byte

func stablePCRSeed() []byte {
	if len(pcrSeed) > 0 {
		return pcrSeed
	}
	pcrSeed = make([]byte, 32)
	_, _ = rand.Read(pcrSeed)
	return pcrSeed
}

func bytesEqual(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// SaveSealedARK 把 SealedARK 写到文件。
func SaveSealedARK(path string, sealed *SealedARK) error {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return err
	}
	defer f.Close()
	return gob.NewEncoder(f).Encode(sealed)
}

// LoadSealedARK 从文件加载。
func LoadSealedARK(path string) (*SealedARK, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var s SealedARK
	if err := gob.NewDecoder(f).Decode(&s); err != nil {
		return nil, err
	}
	return &s, nil
}

