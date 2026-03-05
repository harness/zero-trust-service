package verifier

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"git0.harness.io/l7B_kbSEQD2wjrM7PShm5w/PROD/Harness_Commons/zero-trust-service/metrics"
	"git0.harness.io/l7B_kbSEQD2wjrM7PShm5w/PROD/Harness_Commons/zero-trust-service/resolver"
	"git0.harness.io/l7B_kbSEQD2wjrM7PShm5w/PROD/Harness_Commons/zero-trust-service/types"
)

type resolvedPipelineKey struct{}

// PipelineHolder is a mutable container stored in the context so that the
// resolver middleware can share the resolved pipeline with downstream validators.
type PipelineHolder struct {
	pipeline *resolver.ResolvedPipeline
}

func (h *PipelineHolder) set(rp *resolver.ResolvedPipeline) { h.pipeline = rp }

// Get returns the resolved pipeline, or nil if resolution hasn't run / failed.
func (h *PipelineHolder) Get() *resolver.ResolvedPipeline { return h.pipeline }

// WithPipelineHolder stores a PipelineHolder in the context.
func WithPipelineHolder(ctx context.Context) (context.Context, *PipelineHolder) {
	h := &PipelineHolder{}
	return context.WithValue(ctx, resolvedPipelineKey{}, h), h
}

// PipelineHolderFrom retrieves the PipelineHolder from the context.
func PipelineHolderFrom(ctx context.Context) *PipelineHolder {
	h, _ := ctx.Value(resolvedPipelineKey{}).(*PipelineHolder)
	return h
}

// ResolvedPipelineFrom is a convenience that returns the resolved pipeline
// directly from the context, or nil if unavailable.
func ResolvedPipelineFrom(ctx context.Context) *resolver.ResolvedPipeline {
	h := PipelineHolderFrom(ctx)
	if h == nil {
		return nil
	}
	return h.Get()
}

// ResolverMiddleware resolves the pipeline YAML and stores the result in the
// context via PipelineHolder. Resolution errors are logged but never fail the chain.
type ResolverMiddleware struct {
	resolver  *resolver.Resolver
	metrics   *metrics.Metrics
	qualifyFn func(repo string) string
	outputDir string
}

type ResolverMiddlewareOption func(*ResolverMiddleware)

func WithRepoQualifier(fn func(repo string) string) ResolverMiddlewareOption {
	return func(rm *ResolverMiddleware) { rm.qualifyFn = fn }
}

// WithOutputDir configures the middleware to write resolved pipeline YAML
// to the given directory. The file is named after the execution/task ID.
func WithOutputDir(dir string) ResolverMiddlewareOption {
	return func(rm *ResolverMiddleware) { rm.outputDir = dir }
}

func NewResolverMiddleware(r *resolver.Resolver, m *metrics.Metrics, opts ...ResolverMiddlewareOption) *ResolverMiddleware {
	rm := &ResolverMiddleware{resolver: r, metrics: m}
	for _, o := range opts {
		o(rm)
	}
	return rm
}

func (rm *ResolverMiddleware) Handle(ctx context.Context, request types.VerifyRequest) error {
	result := rm.tryResolve(ctx, request)
	if result == nil {
		return nil
	}
	if h := PipelineHolderFrom(ctx); h != nil {
		h.set(result)
	}
	return nil
}

func (rm *ResolverMiddleware) qualifyRepo(repo string) string {
	if rm.qualifyFn != nil {
		return rm.qualifyFn(repo)
	}
	return repo
}

func (rm *ResolverMiddleware) tryResolve(ctx context.Context, request types.VerifyRequest) *resolver.ResolvedPipeline {
	m := rm.metrics
	meta := request.TaskPackage
	if meta == nil || meta.ZTSMetadata == nil {
		return nil
	}

	zts := meta.ZTSMetadata
	git := zts.PipelineGitDetails

	if git == nil || git.RepoName == "" || git.FilePath == "" {
		m.ResolverTotal.Inc(metrics.LabelResolverInline)
		return nil
	}

	exec := zts.ExecutionDetails
	fileID := execID(exec, meta.TaskID)

	ref := git.Branch
	if git.CommitID != "" {
		ref = git.CommitID
	}

	qualifiedRepo := rm.qualifyRepo(git.RepoName)

	resolveCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	log.Printf("[resolver] resolving pipeline execution=%s repo=%s file=%s ref=%s",
		fileID, qualifiedRepo, git.FilePath, ref)

	start := time.Now()
	result, err := rm.resolver.LoadAndResolvePipeline(resolveCtx,
		zts.AccountID, zts.OrgIdentifier, zts.ProjectIdentifier, zts.ParentUniqueID,
		resolver.FileRef{
			Repo: qualifiedRepo,
			Path: git.FilePath,
			Ref:  ref,
		},
	)
	dur := time.Since(start)

	if err != nil {
		m.ResolverTotal.Inc(metrics.LabelResolverError)
		m.ResolverDuration.Observe(dur.Seconds(), metrics.LabelResolverError)
		log.Printf("[resolver] failed execution=%s duration=%s: %v", fileID, dur, err)
		return nil
	}

	m.ResolverTotal.Inc(metrics.LabelResolverSuccess)
	m.ResolverDuration.Observe(dur.Seconds(), metrics.LabelResolverSuccess)

	if rm.outputDir != "" {
		rm.writeResolved(fileID, result.ResolvedYAML)
	}

	log.Printf("[resolver] resolved pipeline execution=%s duration=%s templates_used=%d",
		fileID, dur, len(result.TemplatesUsed))
	return result
}

func (rm *ResolverMiddleware) writeResolved(fileID, yaml string) {
	if err := os.MkdirAll(rm.outputDir, 0o755); err != nil {
		log.Printf("[resolver] failed to create output dir %s: %v", rm.outputDir, err)
		return
	}
	outPath := filepath.Join(rm.outputDir, fileID+".yaml")
	if err := os.WriteFile(outPath, []byte(yaml), 0o644); err != nil {
		log.Printf("[resolver] failed to write resolved yaml to %s: %v", outPath, err)
		return
	}
	log.Printf("[resolver] resolved pipeline written to %s", outPath)
}

func execID(exec *types.ExecutionDetails, taskID string) string {
	if exec != nil && exec.PipelineExecutionID != "" {
		return exec.PipelineExecutionID
	}
	if taskID != "" {
		return taskID
	}
	return fmt.Sprintf("unknown-%d", time.Now().UnixMilli())
}
