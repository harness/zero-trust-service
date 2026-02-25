package types

import "testing"

func TestResolveAccountID_PreferZTSMetadata(t *testing.T) {
	req := VerifyRequest{
		TaskPackage: &DelegateTaskPackage{
			AccountID:   "top-level",
			ZTSMetadata: &ZTSMetadata{AccountID: "zts-meta"},
		},
	}
	if got := req.ResolveAccountID(); got != "zts-meta" {
		t.Errorf("expected zts-meta, got %q", got)
	}
}

func TestResolveAccountID_FallbackToTopLevel(t *testing.T) {
	req := VerifyRequest{
		TaskPackage: &DelegateTaskPackage{
			AccountID: "top-level",
		},
	}
	if got := req.ResolveAccountID(); got != "top-level" {
		t.Errorf("expected top-level, got %q", got)
	}
}

func TestResolveAccountID_EmptyZTSMetadata(t *testing.T) {
	req := VerifyRequest{
		TaskPackage: &DelegateTaskPackage{
			AccountID:   "top-level",
			ZTSMetadata: &ZTSMetadata{AccountID: ""},
		},
	}
	if got := req.ResolveAccountID(); got != "top-level" {
		t.Errorf("expected fallback to top-level, got %q", got)
	}
}

func TestResolveAccountID_AllEmpty(t *testing.T) {
	req := VerifyRequest{}
	if got := req.ResolveAccountID(); got != "" {
		t.Errorf("expected empty, got %q", got)
	}
}

func TestResolveAccountID_NilTaskPackage(t *testing.T) {
	req := VerifyRequest{TaskPackage: nil}
	if got := req.ResolveAccountID(); got != "" {
		t.Errorf("expected empty, got %q", got)
	}
}

func TestResolveTaskType(t *testing.T) {
	req := VerifyRequest{
		TaskPackage: &DelegateTaskPackage{
			TaskDetails: &TaskDetails{TaskType: "SHELL_SCRIPT"},
		},
	}
	if got := req.ResolveTaskType(); got != "SHELL_SCRIPT" {
		t.Errorf("expected SHELL_SCRIPT, got %q", got)
	}
}

func TestResolveTaskType_NilTaskPackage(t *testing.T) {
	req := VerifyRequest{}
	if got := req.ResolveTaskType(); got != "" {
		t.Errorf("expected empty, got %q", got)
	}
}

func TestResolveTaskType_NilTaskDetails(t *testing.T) {
	req := VerifyRequest{TaskPackage: &DelegateTaskPackage{}}
	if got := req.ResolveTaskType(); got != "" {
		t.Errorf("expected empty, got %q", got)
	}
}
