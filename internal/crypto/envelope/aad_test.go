package envelope

import (
	"bytes"
	"testing"
)

// TestCanonicalDeterministic 验证相同输入应产生相同输出。
func TestCanonicalDeterministic(t *testing.T) {
	parts1 := [][]byte{[]byte("kid-abc"), []byte("v1"), []byte("ctx")}
	parts2 := [][]byte{[]byte("kid-abc"), []byte("v1"), []byte("ctx")}

	c1 := Canonical(parts1...)
	c2 := Canonical(parts2...)

	if !bytes.Equal(c1, c2) {
		t.Fatalf("相同输入应产生相同输出: got %x vs %x", c1, c2)
	}

	// 多次调用应稳定
	c3 := Canonical(parts1...)
	if !bytes.Equal(c1, c3) {
		t.Fatalf("多次调用应稳定: got %x vs %x", c1, c3)
	}
}

// TestCanonicalNoAmbiguity 验证不同输入不应产生相同输出（无歧义）。
func TestCanonicalNoAmbiguity(t *testing.T) {
	// 经典歧义场景：("ab","c") 与 ("a","bc") 不应产生相同 canonical AAD
	a := Canonical([]byte("ab"), []byte("c"))
	b := Canonical([]byte("a"), []byte("bc"))
	if bytes.Equal(a, b) {
		t.Fatalf("歧义拼接: ('ab','c') 与 ('a','bc') 不应产生相同输出, got %x", a)
	}

	// ("abc",) 与 ("a","bc") 不应产生相同输出
	c := Canonical([]byte("abc"))
	if bytes.Equal(c, b) {
		t.Fatalf("歧义拼接: ('abc',) 与 ('a','bc') 不应产生相同输出, got %x", b)
	}

	// 单部分 ("ab",) 与双部分 ("a","b") 不应产生相同输出
	d := Canonical([]byte("ab"))
	e := Canonical([]byte("a"), []byte("b"))
	if bytes.Equal(d, e) {
		t.Fatalf("歧义拼接: ('ab',) 与 ('a','b') 不应产生相同输出, got %x", d)
	}
}

// TestVerify 验证正确和错误的 AAD。
func TestVerify(t *testing.T) {
	parts := [][]byte{[]byte("kid-abc"), []byte("v1")}
	canonical := Canonical(parts...)

	// 正确的 parts 应验证通过
	if !Verify(canonical, parts...) {
		t.Fatal("正确的 parts 应验证通过")
	}

	// 错误的 parts 应验证失败
	wrongParts := [][]byte{[]byte("kid-wrong"), []byte("v1")}
	if Verify(canonical, wrongParts...) {
		t.Fatal("错误的 parts 应验证失败")
	}

	// 顺序不同应验证失败
	reordered := [][]byte{[]byte("v1"), []byte("kid-abc")}
	if Verify(canonical, reordered...) {
		t.Fatal("顺序不同的 parts 应验证失败")
	}

	// 缺少一个 part 应验证失败
	if Verify(canonical, parts[0]) {
		t.Fatal("缺少 part 应验证失败")
	}

	// 多余的 part 应验证失败
	if Verify(canonical, parts[0], parts[1], []byte("extra")) {
		t.Fatal("多余的 part 应验证失败")
	}

	// 空 canonical 与空 parts 应验证通过
	emptyCanonical := Canonical()
	if !Verify(emptyCanonical) {
		t.Fatal("空 canonical 与空 parts 应验证通过")
	}
}

// TestCanonicalEmptyParts 验证空部分处理。
func TestCanonicalEmptyParts(t *testing.T) {
	// 完全无参数应返回空字节串
	c := Canonical()
	if len(c) != 0 {
		t.Fatalf("无参数应返回空字节串, got %x (len=%d)", c, len(c))
	}

	// 包含空字节串的部分应正确编码（长度前缀为 0）
	c = Canonical([]byte(""))
	if len(c) != 1 || c[0] != 0 {
		t.Fatalf("单个空部分应编码为 [0x00], got %x", c)
	}

	// 混合空与非空部分
	c = Canonical([]byte("a"), []byte(""), []byte("bc"))
	// 期望: varint(1) || "a" || varint(0) || varint(2) || "bc"
	expected := []byte{1, 'a', 0, 2, 'b', 'c'}
	if !bytes.Equal(c, expected) {
		t.Fatalf("混合空与非空部分编码不符: got %x, want %x", c, expected)
	}

	// 验证可往返
	if !Verify(c, []byte("a"), []byte(""), []byte("bc")) {
		t.Fatal("混合空与非空部分的 Verify 应通过")
	}
}

// TestCanonicalLengthPrefix 验证长度前缀使用 varint 编码。
func TestCanonicalLengthPrefix(t *testing.T) {
	// 长度 < 128 时 varint 占 1 字节
	c := Canonical([]byte("abc"))
	// 期望: 0x03 || "abc"
	expected := []byte{3, 'a', 'b', 'c'}
	if !bytes.Equal(c, expected) {
		t.Fatalf("短内容编码不符: got %x, want %x", c, expected)
	}

	// 长度 >= 128 时 varint 占 2 字节
	long := bytes.Repeat([]byte{'x'}, 200)
	c = Canonical(long)
	// varint(200) = [0xC8, 0x01]
	if len(c) < 2 {
		t.Fatalf("长内容编码过短: %x", c)
	}
	if c[0] != 0xC8 || c[1] != 0x01 {
		t.Fatalf("varint(200) 编码不符: got %x %x, want 0xC8 0x01", c[0], c[1])
	}
	if !bytes.Equal(c[2:], long) {
		t.Fatal("长内容主体应与输入一致")
	}
}
