// Copyright 2026 Harness, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

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
		TaskPackage: &types.TaskPackage{},
	}, m)
}

func TestRecordMissingMetadata_MissingAccountID(t *testing.T) {
	m := metrics.NewNoop()
	recordMissingMetadata(types.VerifyRequest{
		TaskPackage: &types.TaskPackage{
			ZTSMetadata: &types.ZTSMetadata{},
		},
	}, m)
}

func TestRecordMissingMetadata_MissingTaskType(t *testing.T) {
	m := metrics.NewNoop()
	recordMissingMetadata(types.VerifyRequest{
		TaskPackage: &types.TaskPackage{
			ZTSMetadata: &types.ZTSMetadata{AccountID: "acc1"},
		},
	}, m)
}
