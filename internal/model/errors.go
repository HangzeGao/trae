package model

import "errors"

var (
	ErrTPMNotAvailable      = errors.New("TPM device not available")
	ErrTPMInitFailed        = errors.New("TPM initialization failed")
	ErrKeyNotFound          = errors.New("key not found")
	ErrKeyCreateFailed      = errors.New("key creation failed")
	ErrKeyLoadFailed        = errors.New("key load failed")
	ErrKeyEncryptFailed     = errors.New("key encryption failed")
	ErrKeyDecryptFailed     = errors.New("key decryption failed")
	ErrEncryptFailed        = errors.New("data encryption failed")
	ErrDecryptFailed        = errors.New("data decryption failed")
	ErrInvalidAlgorithm     = errors.New("unsupported algorithm")
	ErrInvalidMode          = errors.New("unsupported cipher mode")
	ErrKeyLengthMismatch    = errors.New("key length does not match algorithm")
	ErrInvalidCiphertext    = errors.New("invalid ciphertext format")
	ErrAuthFailed           = errors.New("authentication failed (GCM tag mismatch)")
	ErrConfigInvalid        = errors.New("invalid configuration")
	ErrKeyServerUnavailable = errors.New("key server unavailable")
)
