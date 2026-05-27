package verify

import (
	"testing"

	"git0.harness.io/l7B_kbSEQD2wjrM7PShm5w/PROD/Harness_Commons/zero-trust-service/metrics"
	"git0.harness.io/l7B_kbSEQD2wjrM7PShm5w/PROD/Harness_Commons/zero-trust-service/types"
)

func TestRecordMissingMetadata_NilTaskPackage(t *testing.T) {
	m := metrics.NewNoop()
	recordMissingMetadata(types.VerifyRequest{}, m)
}

func TestRecordMissingMetadata_NilZTSMetadata(t *testing.T) {
	m := metrics.NewNoop()
	recordMissingMetadata(types.VerifyRequest{
		TaskPackage: &types.DelegateTaskPackage{},
	}, m)
}

func TestRecordMissingMetadata_MissingAccountID(t *testing.T) {
	m := metrics.NewNoop()
	recordMissingMetadata(types.VerifyRequest{
		TaskPackage: &types.DelegateTaskPackage{
			ZTSMetadata: &types.ZTSMetadata{},
		},
	}, m)
}

func TestRecordMissingMetadata_MissingTaskType(t *testing.T) {
	m := metrics.NewNoop()
	recordMissingMetadata(types.VerifyRequest{
		TaskPackage: &types.DelegateTaskPackage{
			ZTSMetadata: &types.ZTSMetadata{AccountID: "acc1"},
		},
	}, m)
}
