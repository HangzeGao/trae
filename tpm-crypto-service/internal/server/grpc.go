package server

import (
	"context"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	tpmcrypto "github.com/tpm-crypto/tpm-crypto-service/api/gen/go"
	"github.com/tpm-crypto/tpm-crypto-service/internal/crypto/common"
	"github.com/tpm-crypto/tpm-crypto-service/internal/keystore"
	"github.com/tpm-crypto/tpm-crypto-service/internal/service"
)

type grpcImpl struct {
	tpmcrypto.UnimplementedCryptoServiceServer
	svc *service.Service
}

func (g *grpcImpl) CreateDataKey(ctx context.Context, req *tpmcrypto.CreateDataKeyRequest) (*tpmcrypto.DataKeyMeta, error) {
	alg, err := mapAlgorithm(req.Algorithm)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	m, err := g.svc.CreateDataKey(ctx, req.KeyId, alg, req.KeyLengthBits)
	if err != nil {
		return nil, mapErr(err)
	}
	return toProto(m), nil
}

func (g *grpcImpl) DescribeDataKey(ctx context.Context, req *tpmcrypto.DescribeDataKeyRequest) (*tpmcrypto.DataKeyMeta, error) {
	m, err := g.svc.DescribeDataKey(ctx, req.KeyId, req.Version)
	if err != nil {
		return nil, mapErr(err)
	}
	return toProto(m), nil
}

func (g *grpcImpl) RotateDataKey(ctx context.Context, req *tpmcrypto.RotateDataKeyRequest) (*tpmcrypto.DataKeyMeta, error) {
	m, err := g.svc.RotateDataKey(ctx, req.KeyId)
	if err != nil {
		return nil, mapErr(err)
	}
	return toProto(m), nil
}

func (g *grpcImpl) RevokeDataKey(ctx context.Context, req *tpmcrypto.RevokeDataKeyRequest) (*tpmcrypto.RevokeDataKeyResponse, error) {
	n, err := g.svc.RevokeDataKey(ctx, req.KeyId, req.Version)
	if err != nil {
		return nil, mapErr(err)
	}
	return &tpmcrypto.RevokeDataKeyResponse{RevokedVersions: n}, nil
}

func (g *grpcImpl) Encrypt(ctx context.Context, req *tpmcrypto.EncryptRequest) (*tpmcrypto.EncryptResponse, error) {
	alg, err := mapAlgorithm(req.Algorithm)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	mode, err := mapMode(req.Mode)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	ct, iv, tag, usedV, err := g.svc.Encrypt(ctx, req.KeyId, req.KeyVersion, alg, mode, req.Plaintext, req.Aad)
	if err != nil {
		return nil, mapErr(err)
	}
	return &tpmcrypto.EncryptResponse{
		Ciphertext: ct,
		Iv:         iv,
		Tag:        tag,
		KeyVersion: usedV,
	}, nil
}

func (g *grpcImpl) Decrypt(ctx context.Context, req *tpmcrypto.DecryptRequest) (*tpmcrypto.DecryptResponse, error) {
	alg, err := mapAlgorithm(req.Algorithm)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	mode, err := mapMode(req.Mode)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	pt, err := g.svc.Decrypt(ctx, req.KeyId, req.KeyVersion, alg, mode, req.Ciphertext, req.Iv, req.Tag, req.Aad)
	if err != nil {
		return nil, mapErr(err)
	}
	return &tpmcrypto.DecryptResponse{Plaintext: pt}, nil
}

func (g *grpcImpl) HealthCheck(_ context.Context, _ *tpmcrypto.HealthCheckRequest) (*tpmcrypto.HealthCheckResponse, error) {
	h := g.svc.HealthCheck(nil)
	ok := h.TPMReady && h.ARKReady && h.KSReady
	resp := &tpmcrypto.HealthCheckResponse{
		Components: map[string]bool{
			"tpm":  h.TPMReady,
			"ark":  h.ARKReady,
			"ks":   h.KSReady,
			"mode": h.Mode == "simulator",
		},
	}
	if ok {
		resp.Status = tpmcrypto.HealthCheckResponse_SERVING
		return resp, nil
	}
	resp.Status = tpmcrypto.HealthCheckResponse_NOT_SERVING
	resp.Message = "tpm/ark/ks not ready"
	return resp, status.Error(codes.Unavailable, resp.Message)
}

func mapAlgorithm(p tpmcrypto.Algorithm) (common.Algorithm, error) {
	switch p {
	case tpmcrypto.Algorithm_ALGORITHM_AES:
		return common.AES, nil
	case tpmcrypto.Algorithm_ALGORITHM_SM4:
		return common.SM4, nil
	}
	return common.AlgUnknown, status.Error(codes.InvalidArgument, "unsupported algorithm")
}

func mapMode(p tpmcrypto.BlockMode) (common.BlockMode, error) {
	switch p {
	case tpmcrypto.BlockMode_BLOCK_MODE_ECB:
		return common.ECB, nil
	case tpmcrypto.BlockMode_BLOCK_MODE_CBC:
		return common.CBC, nil
	case tpmcrypto.BlockMode_BLOCK_MODE_CTR:
		return common.CTR, nil
	case tpmcrypto.BlockMode_BLOCK_MODE_CFB:
		return common.CFB, nil
	case tpmcrypto.BlockMode_BLOCK_MODE_OFB:
		return common.OFB, nil
	case tpmcrypto.BlockMode_BLOCK_MODE_GCM:
		return common.GCM, nil
	}
	return common.ModeUnknown, status.Error(codes.InvalidArgument, "unsupported block mode")
}

func mapStatus(s common.KeyStatus) tpmcrypto.KeyStatus {
	switch s {
	case common.Active:
		return tpmcrypto.KeyStatus_KEY_STATUS_ACTIVE
	case common.Retired:
		return tpmcrypto.KeyStatus_KEY_STATUS_RETIRED
	case common.Revoked:
		return tpmcrypto.KeyStatus_KEY_STATUS_REVOKED
	}
	return tpmcrypto.KeyStatus_KEY_STATUS_UNSPECIFIED
}

func toProto(m *keystore.DataKeyMeta) *tpmcrypto.DataKeyMeta {
	return &tpmcrypto.DataKeyMeta{
		KeyId:         m.KeyID,
		Algorithm:     algToProto(m.Algorithm),
		KeyLengthBits: m.KeyLengthBits,
		Version:       m.Version,
		Status:        mapStatus(m.Status),
		CreatedAt:     timestamppb.New(timeOrZero(m.CreatedAt)),
	}
}

func algToProto(a common.Algorithm) tpmcrypto.Algorithm {
	switch a {
	case common.AES:
		return tpmcrypto.Algorithm_ALGORITHM_AES
	case common.SM4:
		return tpmcrypto.Algorithm_ALGORITHM_SM4
	}
	return tpmcrypto.Algorithm_ALGORITHM_UNSPECIFIED
}

func timeOrZero(t time.Time) time.Time {
	if t.IsZero() {
		return time.Unix(0, 0)
	}
	return t
}

func mapErr(err error) error {
	if err == nil {
		return nil
	}
	switch {
	case errIs(err, common.ErrNotFound):
		return status.Error(codes.NotFound, err.Error())
	case errIs(err, common.ErrKeyIDExists):
		return status.Error(codes.AlreadyExists, err.Error())
	case errIs(err, common.ErrKeyRevoked):
		return status.Error(codes.PermissionDenied, err.Error())
	case errIs(err, common.ErrInvalidArgument):
		return status.Error(codes.InvalidArgument, err.Error())
	case errIs(err, common.ErrAuth):
		return status.Error(codes.PermissionDenied, err.Error())
	case errIs(err, common.ErrUnsupported):
		return status.Error(codes.Unimplemented, err.Error())
	}
	return status.Error(codes.Internal, err.Error())
}

func errIs(err error, targets ...error) bool {
	for _, t := range targets {
		if errorsIs(err, t) {
			return true
		}
	}
	return false
}

// errorsIs 是 errors.Is 的薄包装,避免在 import 里硬引用 stdlib errors。
func errorsIs(err, target error) bool {
	for err != nil {
		if err == target {
			return true
		}
		type unwrapper interface{ Unwrap() error }
		if u, ok := err.(unwrapper); ok {
			err = u.Unwrap()
		} else {
			return false
		}
	}
	return false
}
