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

const (
	metricResolverDuration = "zts_resolver_duration_seconds"
	metricResolverTotal    = "zts_resolver_total"

	resolverInline  = "inline"
	resolverSuccess = "success"
	resolverError   = "error"

	keyStatus    = "status"
	keyAccountID = "account_id"
)

type resolvedPipelineKey struct{}

// PipelineHolder is a mutable container stored in the context so that the
// resolver can share the resolved pipeline with downstream verifiers.
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

// Resolver resolves pipeline YAML from SCM and stores the result in the
// context via PipelineHolder before delegating to the next verifier.
type Resolver struct {
	resolver  *resolver.Resolver
	metrics   metrics.Emitter
	qualifyFn func(repo string) string
	outputDir string
}

type ResolverOption func(*Resolver)

func WithRepoQualifier(fn func(repo string) string) ResolverOption {
	return func(r *Resolver) { r.qualifyFn = fn }
}

// WithOutputDir configures the resolver to write resolved pipeline YAML
// to the given directory.
func WithOutputDir(dir string) ResolverOption {
	return func(r *Resolver) { r.outputDir = dir }
}

func NewResolver(r *resolver.Resolver, m metrics.Emitter, opts ...ResolverOption) *Resolver {
	res := &Resolver{resolver: r, metrics: m}
	for _, o := range opts {
		o(res)
	}
	return res
}

// Wrap returns a new verifier that resolves the pipeline, stores it
// in context, then calls next.Handle.
func (rm *Resolver) Wrap(next Interface) Interface {
	return From(func(ctx context.Context, request types.VerifyRequest) error {
		ctx, holder := WithPipelineHolder(ctx)
		result := rm.tryResolve(ctx, request)
		if result != nil {
			holder.set(result)
		}
		return next.Handle(ctx, request)
	})
}

func (rm *Resolver) qualifyRepo(repo string) string {
	if rm.qualifyFn != nil {
		return rm.qualifyFn(repo)
	}
	return repo
}

func (rm *Resolver) tryResolve(ctx context.Context, request types.VerifyRequest) *resolver.ResolvedPipeline {
	m := rm.metrics
	meta := request.TaskPackage
	if meta == nil || meta.ZTSMetadata == nil {
		return nil
	}

	zts := meta.ZTSMetadata
	git := zts.PipelineGitDetails

	accountID := zts.AccountID

	if git == nil || git.RepoName == "" || git.FilePath == "" {
		m.Counter(metricResolverTotal, 1, metrics.Dim(keyStatus, resolverInline), metrics.Dim(keyAccountID, accountID))
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
		m.Counter(metricResolverTotal, 1, metrics.Dim(keyStatus, resolverError), metrics.Dim(keyAccountID, accountID))
		m.Histogram(metricResolverDuration, dur.Seconds(), metrics.Dim(keyStatus, resolverError), metrics.Dim(keyAccountID, accountID))
		log.Printf("[resolver] failed execution=%s duration=%s: %v", fileID, dur, err)
		return nil
	}

	m.Counter(metricResolverTotal, 1, metrics.Dim(keyStatus, resolverSuccess), metrics.Dim(keyAccountID, accountID))
	m.Histogram(metricResolverDuration, dur.Seconds(), metrics.Dim(keyStatus, resolverSuccess), metrics.Dim(keyAccountID, accountID))

	if rm.outputDir != "" {
		rm.writeResolved(fileID, result.ResolvedYAML)
	}

	log.Printf("[resolver] resolved pipeline execution=%s duration=%s templates_used=%d",
		fileID, dur, len(result.TemplatesUsed))
	return result
}

func (rm *Resolver) writeResolved(fileID, yaml string) {
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
