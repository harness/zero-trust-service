package verifier

import (
	"context"
	"os"
	"testing"

	"git0.harness.io/l7B_kbSEQD2wjrM7PShm5w/PROD/Harness_Commons/zero-trust-service/metrics"
	"git0.harness.io/l7B_kbSEQD2wjrM7PShm5w/PROD/Harness_Commons/zero-trust-service/resolver"
	"git0.harness.io/l7B_kbSEQD2wjrM7PShm5w/PROD/Harness_Commons/zero-trust-service/types"
)

// ---- context helpers --------------------------------------------------------

func TestWithPipelineHolder_RoundTrip(t *testing.T) {
	ctx, h := WithPipelineHolder(context.Background())
	if h == nil {
		t.Fatal("expected non-nil holder")
	}
	got := PipelineHolderFrom(ctx)
	if got != h {
		t.Errorf("PipelineHolderFrom returned different holder")
	}
}

func TestPipelineHolderFrom_Missing(t *testing.T) {
	got := PipelineHolderFrom(context.Background())
	if got != nil {
		t.Errorf("expected nil for context without holder, got %v", got)
	}
}

func TestResolvedPipelineFrom_NoHolder(t *testing.T) {
	rp := ResolvedPipelineFrom(context.Background())
	if rp != nil {
		t.Errorf("expected nil, got %v", rp)
	}
}

func TestResolvedPipelineFrom_EmptyHolder(t *testing.T) {
	ctx, _ := WithPipelineHolder(context.Background())
	rp := ResolvedPipelineFrom(ctx)
	if rp != nil {
		t.Errorf("expected nil before set, got %v", rp)
	}
}

func TestPipelineHolder_SetAndGet(t *testing.T) {
	_, h := WithPipelineHolder(context.Background())
	rp := &resolver.ResolvedPipeline{ResolvedYAML: "pipeline: {}"}
	h.set(rp)
	if h.Get() != rp {
		t.Error("Get() did not return the value set via set()")
	}
}

func TestResolvedPipelineFrom_AfterSet(t *testing.T) {
	ctx, h := WithPipelineHolder(context.Background())
	rp := &resolver.ResolvedPipeline{ResolvedYAML: "pipeline: {}"}
	h.set(rp)
	got := ResolvedPipelineFrom(ctx)
	if got != rp {
		t.Errorf("expected %v, got %v", rp, got)
	}
}

// ---- Resolver options -------------------------------------------------------

func TestNewResolver_Defaults(t *testing.T) {
	r := NewResolver(nil, metrics.NewNoop())
	if r == nil {
		t.Fatal("expected non-nil Resolver")
	}
	if r.qualifyFn != nil {
		t.Error("expected nil qualifyFn by default")
	}
	if r.outputDir != "" {
		t.Error("expected empty outputDir by default")
	}
}

func TestWithRepoQualifier(t *testing.T) {
	fn := func(repo string) string { return "qualified/" + repo }
	r := NewResolver(nil, metrics.NewNoop(), WithRepoQualifier(fn))
	if got := r.qualifyRepo("my-repo"); got != "qualified/my-repo" {
		t.Errorf("expected qualified/my-repo, got %q", got)
	}
}

func TestWithOutputDir(t *testing.T) {
	r := NewResolver(nil, metrics.NewNoop(), WithOutputDir("/tmp/resolved"))
	if r.outputDir != "/tmp/resolved" {
		t.Errorf("expected /tmp/resolved, got %q", r.outputDir)
	}
}

func TestQualifyRepo_NoFn(t *testing.T) {
	r := NewResolver(nil, metrics.NewNoop())
	if got := r.qualifyRepo("repo"); got != "repo" {
		t.Errorf("expected repo unchanged, got %q", got)
	}
}

// ---- tryResolve nil-metadata fast-path -------------------------------------

func TestTryResolve_NilResult(t *testing.T) {
	tests := []struct {
		name string
		req  types.VerifyRequest
	}{
		{"nil task package", types.VerifyRequest{}},
		{"nil zts metadata", types.VerifyRequest{TaskPackage: &types.TaskPackage{}}},
		{"no git details", types.VerifyRequest{TaskPackage: &types.TaskPackage{
			ZTSMetadata: &types.ZTSMetadata{AccountID: "acc1"},
		}}},
		{"empty repo name", types.VerifyRequest{TaskPackage: &types.TaskPackage{
			ZTSMetadata: &types.ZTSMetadata{
				AccountID:          "acc1",
				PipelineGitDetails: &types.GitDetails{RepoName: "", FilePath: "pipeline.yaml"},
			},
		}}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			r := NewResolver(nil, metrics.NewNoop())
			if got := r.tryResolve(context.Background(), tc.req); got != nil {
				t.Errorf("expected nil, got %v", got)
			}
		})
	}
}

// ---- Wrap delegates to next -------------------------------------------------

func TestResolver_Wrap_NilMetadata_CallsNext(t *testing.T) {
	r := NewResolver(nil, metrics.NewNoop())
	called := false
	next := From(func(_ context.Context, _ types.VerifyRequest) error {
		called = true
		return nil
	})
	wrapped := r.Wrap(next)
	_ = wrapped.Handle(context.Background(), types.VerifyRequest{})
	if !called {
		t.Error("next verifier was not called")
	}
}

// ---- execID -----------------------------------------------------------------

func TestExecID_FromExecution(t *testing.T) {
	exec := &types.ExecutionDetails{PipelineExecutionID: "exec-123"}
	if got := execID(exec, "task-456"); got != "exec-123" {
		t.Errorf("expected exec-123, got %q", got)
	}
}

func TestExecID_FallbackToTaskID(t *testing.T) {
	if got := execID(nil, "task-456"); got != "task-456" {
		t.Errorf("expected task-456, got %q", got)
	}
}

func TestExecID_FallbackUnknown(t *testing.T) {
	if got := execID(nil, ""); got == "" {
		t.Error("expected non-empty fallback id")
	}
}

func TestWriteResolved_CreatesFile(t *testing.T) {
	dir := t.TempDir()
	r := NewResolver(&resolver.Resolver{}, metrics.NewNoop(), WithOutputDir(dir))
	r.writeResolved("exec-1", "pipeline:\n  identifier: p1\n")

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	if len(entries) != 1 || entries[0].Name() != "exec-1.yaml" {
		t.Errorf("expected exec-1.yaml, got %v", entries)
	}
}

func TestWriteResolved_BadDir(t *testing.T) {
	// Pass a file path as output dir — MkdirAll will fail, should not panic.
	f, err := os.CreateTemp(t.TempDir(), "*.txt")
	if err != nil {
		t.Fatal(err)
	}
	_ = f.Close()
	r := NewResolver(&resolver.Resolver{}, metrics.NewNoop(), WithOutputDir(f.Name()+"/sub"))
	r.writeResolved("exec-1", "yaml") // should log and return without panic
}

