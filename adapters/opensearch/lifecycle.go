package opensearch

import (
	"context"
	"encoding/json"
	"errors"
	"math/bits"
	"net/http"
	"sync/atomic"
	"unicode/utf8"

	"github.com/faustbrian/go-search"
)

var (
	// ErrLifecycleRejected rejects incomplete, unacknowledged, or inconsistent lifecycle evidence.
	ErrLifecycleRejected = errors.New("search/opensearch: lifecycle response rejected")
	// ErrLifecycleDisabled rejects lifecycle use without configuration.
	ErrLifecycleDisabled = errors.New("search/opensearch: lifecycle is not configured")
	// ErrLifecycleDenied hides non-context application authorization failures.
	ErrLifecycleDenied = errors.New("search/opensearch: lifecycle operation denied")
	// ErrLifecycleVerifierRequired requires semantic/live-definition verification beyond
	// separate backend counts.
	ErrLifecycleVerifierRequired = errors.New("search/opensearch: semantic lifecycle verifier is required")
	// ErrLifecycleCutoverGuardRequired requires application-owned durable write quiescence for
	// cutover.
	ErrLifecycleCutoverGuardRequired = errors.New("search/opensearch: lifecycle cutover guard is required")
	// ErrLifecycleCutoverGuardRejected identifies violated callback/exclusion contract;
	// effects may require reconciliation.
	ErrLifecycleCutoverGuardRejected = errors.New("search/opensearch: lifecycle cutover guard violated its contract")
	// ErrLifecycleCutoverUnverified refuses alias dispatch when fresh semantic verification
	// reports drift.
	ErrLifecycleCutoverUnverified = errors.New("search/opensearch: lifecycle cutover target is not verified")
	// ErrLifecycleMutationGuardRequired requires shared durable resource exclusion for guarded
	// mutations.
	ErrLifecycleMutationGuardRequired = errors.New("search/opensearch: lifecycle mutation guard is required")
	// ErrLifecycleMutationGuardRejected identifies violated mutation callback contract without
	// proving effects were undone.
	ErrLifecycleMutationGuardRejected = errors.New("search/opensearch: lifecycle mutation guard violated its contract")
	// ErrLifecycleCleanupGuardRequired requires final durable eligibility; direct DeleteIndex
	// always refuses.
	ErrLifecycleCleanupGuardRequired = errors.New("search/opensearch: lifecycle cleanup guard is required")
	// ErrLifecycleCleanupGuardRejected identifies violated final-cleanup callback contract,
	// possibly with unknown deletion receipt.
	ErrLifecycleCleanupGuardRejected = errors.New("search/opensearch: lifecycle cleanup guard violated its contract")
)

// LifecycleAuthorizer is a borrowed application policy for exact tenant/resource scope; it
// must be concurrency-safe and cooperate with context.
type LifecycleAuthorizer interface {
	// Authorize checks sensitive tenant/resource scope synchronously before network requests;
	// non-context errors are hidden.
	Authorize(context.Context, string, []string) error
}

// LifecycleAuthorizerFunc adapts a function to LifecycleAuthorizer; nil invocation panics.
type LifecycleAuthorizerFunc func(context.Context, string, []string) error

// Authorize calls the wrapped policy with an independent resource slice; nil functions panic.
func (authorize LifecycleAuthorizerFunc) Authorize(ctx context.Context, tenant string, resources []string) error {
	return authorize(ctx, tenant, append([]string(nil), resources...))
}

// LifecycleVerificationRequest binds semantic comparison to the authorized
// tenant and physical generations whose bounded counts were read immediately
// before verification.
type LifecycleVerificationRequest struct {
	// Tenant scopes application authority; Source and Target identify distinct physical
	// generations.
	Tenant, Source, Target string
	// SourceCount and TargetCount are separate fresh backend count observations, not semantic
	// equivalence.
	SourceCount, TargetCount uint64
	// ExpectedTargetFingerprint is the planned definition identity to attest independently.
	ExpectedTargetFingerprint string
}

// LifecycleVerificationResult reports the live target-definition fingerprint
// and the number of IDs whose presence, external version, or canonical source
// digest differs between generations.
type LifecycleVerificationResult struct {
	// TargetFingerprint must be independently derived from the live target definition, not
	// echoed from input.
	TargetFingerprint string
	// Drift counts differing IDs/versions/canonical sources and must fit count-consistency bounds.
	Drift uint64
}

// LifecycleVerifier performs a stable, bounded semantic comparison of two
// physical generations and derives TargetFingerprint from the live target
// mappings and settings. Implementations must reject counts beyond their hard
// traversal limit, must not sample when authorizing alias cutover, and must not
// echo ExpectedTargetFingerprint without independently attesting live state.
type LifecycleVerifier interface {
	// Verify performs complete bounded stable semantic comparison and live-definition
	// attestation; callback failures are sanitized.
	Verify(context.Context, LifecycleVerificationRequest) (LifecycleVerificationResult, error)
}

// LifecycleVerifierFunc adapts a function to LifecycleVerifier.
type LifecycleVerifierFunc func(context.Context, LifecycleVerificationRequest) (LifecycleVerificationResult, error)

// Verify delegates directly to the wrapped verifier; nil functions panic.
func (verify LifecycleVerifierFunc) Verify(ctx context.Context, request LifecycleVerificationRequest) (LifecycleVerificationResult, error) {
	return verify(ctx, request)
}

var _ search.LifecycleBackend = (*Client)(nil)

// LifecycleCutoverRequest identifies the application write scope that must be
// quiesced continuously across final semantic verification and alias cutover.
type LifecycleCutoverRequest struct {
	// Tenant selects policy; Alias moves from physical Source to physical Target under the
	// application write fence.
	Tenant, Alias, Source, Target string
	// ExpectedTargetFingerprint identifies the target definition requiring fresh attestation.
	ExpectedTargetFingerprint string
}

// LifecycleCutoverGuard coordinates an application-owned durable write fence.
// WithWritesQuiesced must synchronously invoke operation exactly once after all
// in-flight source writes have drained and before new writes can enter. It must
// keep that fence active until operation returns. Implementations must use an
// ingress gate, durable buffer, or equivalent quiescence mechanism; they must
// not hold a Go lock across the callback's OpenSearch network operations.
type LifecycleCutoverGuard interface {
	// WithWritesQuiesced synchronously calls operation once after writes drain, retaining
	// durable quiescence until return; callback-count checks cannot prove that fence.
	WithWritesQuiesced(context.Context, LifecycleCutoverRequest, func() error) error
}

// LifecycleCutoverGuardFunc adapts a function to LifecycleCutoverGuard.
type LifecycleCutoverGuardFunc func(context.Context, LifecycleCutoverRequest, func() error) error

// WithWritesQuiesced delegates directly; the function must satisfy durable synchronous
// single-callback obligations.
func (guard LifecycleCutoverGuardFunc) WithWritesQuiesced(ctx context.Context, request LifecycleCutoverRequest, operation func() error) error {
	return guard(ctx, request, operation)
}

// LifecycleMutationRequest identifies one authorized index or alias mutation
// and every physical or logical resource whose lifecycle state it can change.
type LifecycleMutationRequest struct {
	// Tenant scopes application-owned exclusion.
	Tenant string
	// Operation identifies the mutation family.
	Operation Operation
	// Resources is a copied complete resource set that must share cross-instance exclusion.
	Resources []string
}

// LifecycleMutationGuard coordinates application-owned durable exclusion
// across index creation, alias changes, cutover, and cleanup. Implementations
// must synchronously invoke operation exactly once while holding the same
// cross-instance fence for overlapping tenant resources. A process-local lock
// is not sufficient for production deployments.
type LifecycleMutationGuard interface {
	// WithLifecycleMutation synchronously calls operation once under durable cross-instance
	// resource exclusion; it must propagate the operation result.
	WithLifecycleMutation(context.Context, LifecycleMutationRequest, func() error) error
}

// LifecycleMutationGuardFunc adapts a function to LifecycleMutationGuard.
type LifecycleMutationGuardFunc func(context.Context, LifecycleMutationRequest, func() error) error

// WithLifecycleMutation delegates with an independent resource slice; no durable fence is
// implemented by the wrapper.
func (guard LifecycleMutationGuardFunc) WithLifecycleMutation(ctx context.Context, request LifecycleMutationRequest, operation func() error) error {
	request.Resources = append([]string(nil), request.Resources...)
	return guard(ctx, request, operation)
}

// LifecycleCleanupGuard owns the final durable deletion-eligibility fence.
// It must synchronously invoke operation exactly once while continuously
// proving that no alias, retained reader/PIT, retention, or backup prerequisite
// protects the generation.
type LifecycleCleanupGuard interface {
	// WithCleanupEligible synchronously calls operation once while retaining complete final
	// deletion eligibility, including aliases/readers/retention/backup policy.
	WithCleanupEligible(context.Context, search.LifecycleCleanupRequest, func() error) error
}

// LifecycleCleanupGuardFunc adapts a function to LifecycleCleanupGuard.
type LifecycleCleanupGuardFunc func(context.Context, search.LifecycleCleanupRequest, func() error) error

// WithCleanupEligible delegates directly; the wrapper does not implement final eligibility
// or retention policy.
func (guard LifecycleCleanupGuardFunc) WithCleanupEligible(ctx context.Context, request search.LifecycleCleanupRequest, operation func() error) error {
	return guard(ctx, request, operation)
}

// LifecycleConfig borrows application-owned lifecycle policies, fences, verifier,
// and task codec. New requires Authorizer but other missing dependencies fail
// only at their affected operation. Configuration is not durability qualification.
type LifecycleConfig struct {
	// Authorizer is required when lifecycle configuration is supplied and owns resource policy.
	Authorizer LifecycleAuthorizer
	// Verifier is required for VerifyIndex/CutoverAlias semantic attestation.
	Verifier LifecycleVerifier
	// CutoverGuard is required for verified alias movement and owns durable write quiescence.
	CutoverGuard LifecycleCutoverGuard
	// MutationGuard is required for creation/alias/cleanup exclusion across all application
	// instances.
	MutationGuard LifecycleMutationGuard
	// CleanupGuard is required for final irreversible deletion eligibility.
	CleanupGuard LifecycleCleanupGuard
	// ReindexCursorCodec is required for encrypted expiring task continuation; it is borrowed.
	ReindexCursorCodec *ReindexCursorCodec
}

func cloneLifecycleConfig(config *LifecycleConfig) *LifecycleConfig {
	if config == nil {
		return nil
	}
	copyConfig := *config
	return &copyConfig
}

// CreateIndex authorizes and enters the configured durable mutation guard before
// creating the definition. Both metadata and shard acknowledgement are required;
// ambiguous receipt is not retried and requires application recovery.
func (client *Client) CreateIndex(ctx context.Context, tenant string, definition search.IndexDefinition) error {
	if !validLifecycleTenant(tenant) || !indexTargetPattern.MatchString(definition.Name()) {
		return ErrUnsafeIndexTarget
	}
	if err := client.authorizeLifecycle(ctx, tenant, definition.Name()); err != nil {
		return err
	}
	return client.withLifecycleMutation(ctx, LifecycleMutationRequest{
		Tenant: tenant, Operation: OperationCreateIndex, Resources: []string{definition.Name()},
	}, func(operationCtx context.Context) error {
		body, _ := json.Marshal(map[string]any{"settings": definition.Settings(), "mappings": definition.Mappings()})
		response, err := client.executeMutation(operationCtx, OperationCreateIndex, http.MethodPut, "/"+definition.Name(), body, http.StatusOK)
		if err != nil {
			return err
		}
		var acknowledged struct {
			Acknowledged       bool `json:"acknowledged"`
			ShardsAcknowledged bool `json:"shards_acknowledged"`
		}
		if json.Unmarshal(response, &acknowledged) != nil || !acknowledged.Acknowledged || !acknowledged.ShardsAcknowledged {
			return unknownMalformedFailure(OperationCreateIndex, ErrLifecycleRejected)
		}
		return nil
	})
}

// Reindex authorizes exact physical scope and submits one asynchronous external-
// version task when cursor is empty, or polls a decrypted bound task otherwise.
// Successful incomplete polls reseal a fresh bounded lease; persist the replacement
// cursor. Cancellation stops this request, not the server task. No task cancellation,
// deduplication, migration-ID binding, or durable checkpoint is implemented here.
func (client *Client) Reindex(ctx context.Context, tenant, source, target, cursor string) (string, bool, error) {
	if !validLifecycleTenant(tenant) || !indexTargetPattern.MatchString(source) || !indexTargetPattern.MatchString(target) || source == target {
		return cursor, false, ErrUnsafeIndexTarget
	}
	if err := client.authorizeLifecycle(ctx, tenant, source, target); err != nil {
		return cursor, false, err
	}
	if client.lifecycle.ReindexCursorCodec == nil {
		return cursor, false, ErrLifecycleCursorCodecRequired
	}
	if cursor == "" {
		body, _ := json.Marshal(map[string]any{"source": map[string]any{"index": source}, "dest": map[string]any{"index": target, "version_type": "external"}, "conflicts": "abort"})
		response, err := client.executeMutation(ctx, OperationReindex, http.MethodPost, "/_reindex?refresh=true&wait_for_completion=false", body, http.StatusOK)
		if err != nil {
			return "", false, err
		}
		var started struct {
			Task string `json:"task"`
		}
		if json.Unmarshal(response, &started) != nil {
			return "", false, unknownMalformedFailure(OperationReindex, ErrLifecycleRejected)
		}
		if started.Task == "" {
			return "", false, unknownMalformedFailure(OperationReindex, ErrLifecycleRejected)
		}
		if len(started.Task) > 512 {
			return "", false, unknownMalformedFailure(OperationReindex, ErrLifecycleRejected)
		}
		encoded, encodeErr := client.lifecycle.ReindexCursorCodec.encode(tenant, source, target, started.Task)
		if encodeErr != nil {
			return "", false, unknownMalformedFailure(OperationReindex, encodeErr)
		}
		return encoded, false, nil
	}
	taskID, decodeErr := client.lifecycle.ReindexCursorCodec.decode(cursor, tenant, source, target)
	if decodeErr != nil {
		return cursor, false, decodeErr
	}
	response, err := client.execute(ctx, OperationReindex, http.MethodGet, "/_tasks/"+taskID, nil, http.StatusOK)
	if err != nil {
		return cursor, false, err
	}
	var task struct {
		Completed *bool `json:"completed"`
		Response  *struct {
			Total            *uint64           `json:"total"`
			Created          *uint64           `json:"created"`
			Updated          *uint64           `json:"updated"`
			VersionConflicts *uint64           `json:"version_conflicts"`
			Failures         []json.RawMessage `json:"failures"`
		} `json:"response"`
		Error json.RawMessage `json:"error"`
	}
	if json.Unmarshal(response, &task) != nil || task.Completed == nil {
		return cursor, false, malformedFailure(OperationReindex, ErrLifecycleRejected)
	}
	if !*task.Completed {
		if task.Response != nil || len(task.Error) != 0 {
			return cursor, false, malformedFailure(OperationReindex, ErrLifecycleRejected)
		}
		renewed, renewErr := client.lifecycle.ReindexCursorCodec.encode(tenant, source, target, taskID)
		if renewErr != nil {
			return cursor, false, malformedFailure(OperationReindex, renewErr)
		}
		return renewed, false, nil
	}
	if task.Response == nil || task.Response.Total == nil || task.Response.Created == nil || task.Response.Updated == nil ||
		task.Response.VersionConflicts == nil || task.Response.Failures == nil || len(task.Error) > 0 ||
		*task.Response.VersionConflicts != 0 || len(task.Response.Failures) != 0 ||
		*task.Response.Created > *task.Response.Total || *task.Response.Updated != *task.Response.Total-*task.Response.Created {
		return cursor, false, ErrLifecycleRejected
	}
	return cursor, true, nil
}

// VerifyIndex reads separate counts then invokes the application semantic verifier,
// requiring expected live target fingerprint and possible drift counts. The adapter
// cannot independently prove complete traversal or snapshot isolation. A standalone
// call does not quiesce writers; CutoverAlias supplies the configured write fence.
func (client *Client) VerifyIndex(ctx context.Context, tenant, source, target, expectedTargetFingerprint string) (search.VerificationReport, error) {
	if !validLifecycleTenant(tenant) || !indexTargetPattern.MatchString(source) || !indexTargetPattern.MatchString(target) || source == target ||
		!indexFingerprintPattern.MatchString(expectedTargetFingerprint) {
		return search.VerificationReport{}, ErrUnsafeIndexTarget
	}
	if err := client.authorizeLifecycle(ctx, tenant, source, target); err != nil {
		return search.VerificationReport{}, err
	}
	return client.verifyIndex(ctx, tenant, source, target, expectedTargetFingerprint)
}

func (client *Client) verifyIndex(ctx context.Context, tenant, source, target, expectedTargetFingerprint string) (search.VerificationReport, error) {
	sourceCount, err := client.countIndex(ctx, source)
	if err != nil {
		return search.VerificationReport{}, err
	}
	targetCount, err := client.countIndex(ctx, target)
	if err != nil {
		return search.VerificationReport{}, err
	}
	report := search.VerificationReport{SourceCount: sourceCount, TargetCount: targetCount}
	if client.lifecycle.Verifier == nil {
		return report, ErrLifecycleVerifierRequired
	}
	verification, err := client.lifecycle.Verifier.Verify(ctx, LifecycleVerificationRequest{
		Tenant: tenant, Source: source, Target: target,
		SourceCount: sourceCount, TargetCount: targetCount, ExpectedTargetFingerprint: expectedTargetFingerprint,
	})
	if err != nil {
		return report, lifecycleVerifierFailure(err)
	}
	minimumDrift := max(sourceCount, targetCount) - min(sourceCount, targetCount)
	maximumDrift, overflow := bits.Add64(sourceCount, targetCount, 0)
	if overflow != 0 {
		maximumDrift = ^uint64(0)
	}
	if verification.TargetFingerprint != expectedTargetFingerprint || verification.Drift < minimumDrift || verification.Drift > maximumDrift {
		return report, ErrLifecycleRejected
	}
	report.Drift = verification.Drift
	report.Verified = verification.Drift == 0
	return report, nil
}

func lifecycleVerifierFailure(err error) *Failure {
	if errors.Is(err, context.Canceled) {
		return cancelledFailure(OperationVerifyIndex, context.Canceled)
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return cancelledFailure(OperationVerifyIndex, context.DeadlineExceeded)
	}
	return &Failure{
		Operation: OperationVerifyIndex, Category: FailureRejected,
		OutcomeKnown: true, cause: ErrLifecycleRejected,
	}
}

// CutoverAlias holds the configured application write fence across final
// semantic verification and the atomic alias mutation. Writes acknowledged
// before the fence must be visible to the verifier; writes admitted after the
// fence is released must resolve through the new alias generation. Those are
// application guard/verifier/resolver obligations, not independently enforced
// durability. Post-dispatch ambiguity and later checkpoint failure need recovery.
func (client *Client) CutoverAlias(ctx context.Context, tenant, alias, source, target, expectedTargetFingerprint string) (report search.VerificationReport, err error) {
	defer func() {
		if err != nil {
			client.transport.telemetry.signal(ctx, OperationSwapAlias, TelemetryCutoverFailure)
		}
	}()
	if !validLifecycleTenant(tenant) || !indexTargetPattern.MatchString(alias) || !indexTargetPattern.MatchString(source) ||
		!indexTargetPattern.MatchString(target) || source == target || alias == source || alias == target ||
		!indexFingerprintPattern.MatchString(expectedTargetFingerprint) {
		return search.VerificationReport{}, ErrUnsafeIndexTarget
	}
	if err := client.authorizeLifecycle(ctx, tenant, alias, source, target); err != nil {
		return search.VerificationReport{}, err
	}
	if client.lifecycle.CutoverGuard == nil {
		return search.VerificationReport{}, ErrLifecycleCutoverGuardRequired
	}

	request := LifecycleCutoverRequest{
		Tenant: tenant, Alias: alias, Source: source, Target: target,
		ExpectedTargetFingerprint: expectedTargetFingerprint,
	}
	mutationErr := client.withLifecycleMutation(ctx, LifecycleMutationRequest{
		Tenant: tenant, Operation: OperationSwapAlias, Resources: []string{alias, source, target},
	}, func(operationCtx context.Context) error {
		report, err = client.cutoverAliasGuarded(operationCtx, request)
		return err
	})
	return report, mutationErr
}

func (client *Client) cutoverAliasGuarded(ctx context.Context, request LifecycleCutoverRequest) (report search.VerificationReport, err error) {
	tenant, alias, source, target := request.Tenant, request.Alias, request.Source, request.Target
	expectedTargetFingerprint := request.ExpectedTargetFingerprint
	type cutoverResult struct {
		report search.VerificationReport
		err    error
	}
	operationCtx, cancelOperation := context.WithCancel(ctx)
	defer cancelOperation()
	results := make(chan cutoverResult, 1)
	var calls atomic.Uint32
	var active atomic.Bool
	var completed atomic.Bool
	var violated atomic.Bool
	var swapStarted atomic.Bool
	active.Store(true)
	guardErr := client.lifecycle.CutoverGuard.WithWritesQuiesced(operationCtx, request, func() error {
		if calls.Add(1) != 1 {
			violated.Store(true)
			return lifecycleCutoverGuardFailure()
		}
		if !active.Load() {
			violated.Store(true)
			operationErr := lifecycleCutoverGuardFailure()
			completed.Store(true)
			results <- cutoverResult{err: operationErr}
			return operationErr
		}
		report, operationErr := client.verifyIndex(operationCtx, tenant, source, target, expectedTargetFingerprint)
		if operationErr == nil && !report.Verified {
			operationErr = ErrLifecycleCutoverUnverified
		}
		if operationErr == nil && !active.Load() {
			violated.Store(true)
			operationErr = lifecycleCutoverGuardFailure()
		}
		if operationErr == nil {
			swapStarted.Store(true)
			operationErr = client.swapAlias(operationCtx, alias, source, target)
		}
		completed.Store(true)
		results <- cutoverResult{report: report, err: operationErr}
		return operationErr
	})
	completedAtGuardReturn := completed.Load()
	active.Store(false)
	cancelOperation()
	callCount := calls.Load()
	if callCount == 0 {
		return search.VerificationReport{}, lifecycleCutoverGuardFailure()
	}
	result := <-results
	if !completedAtGuardReturn {
		if !swapStarted.Load() || result.err == nil {
			return result.report, lifecycleCutoverGuardFailure()
		}
		return result.report, withLifecycleCutoverGuardFailure(result.err)
	}
	if result.err != nil {
		guardViolated := violated.Load()
		if guardErr != nil {
			if !errors.Is(guardErr, result.err) {
				guardViolated = true
			}
		}
		if guardViolated {
			return result.report, withLifecycleCutoverGuardFailure(result.err)
		}
		return result.report, result.err
	}
	if violated.Load() || guardErr != nil {
		return result.report, lifecycleCutoverGuardFailure()
	}
	return result.report, nil
}

func lifecycleCutoverGuardFailure() *Failure {
	return &Failure{
		Operation: OperationSwapAlias, Category: FailureRejected,
		OutcomeKnown: true, cause: ErrLifecycleCutoverGuardRejected,
	}
}

func withLifecycleCutoverGuardFailure(err error) error {
	var failure *Failure
	if errors.As(err, &failure) {
		copyFailure := *failure
		copyFailure.cause = errors.Join(failure.cause, ErrLifecycleCutoverGuardRejected)
		return &copyFailure
	}
	return errors.Join(lifecycleCutoverGuardFailure(), err)
}

func (client *Client) countIndex(ctx context.Context, index string) (uint64, error) {
	response, err := client.execute(ctx, OperationVerifyIndex, http.MethodGet, "/"+index+"/_count", nil, http.StatusOK)
	if err != nil {
		return 0, err
	}
	var count struct {
		Count  *uint64 `json:"count"`
		Shards *struct {
			Total      *int `json:"total"`
			Successful *int `json:"successful"`
			Failed     *int `json:"failed"`
		} `json:"_shards"`
	}
	if json.Unmarshal(response, &count) != nil || count.Count == nil || count.Shards == nil || count.Shards.Total == nil ||
		count.Shards.Successful == nil || count.Shards.Failed == nil || *count.Shards.Total <= 0 ||
		*count.Shards.Failed != 0 || *count.Shards.Successful != *count.Shards.Total {
		return 0, malformedFailure(OperationVerifyIndex, ErrLifecycleRejected)
	}
	return *count.Count, nil
}

// ResolveAlias authorizes and requires one returned physical index containing the
// requested alias. It is a separate observation, not a compare-and-swap transaction.
func (client *Client) ResolveAlias(ctx context.Context, tenant, alias string) (string, error) {
	if !validLifecycleTenant(tenant) || !indexTargetPattern.MatchString(alias) {
		return "", ErrUnsafeIndexTarget
	}
	if err := client.authorizeLifecycle(ctx, tenant, alias); err != nil {
		return "", err
	}
	response, err := client.execute(ctx, OperationResolveAlias, http.MethodGet, "/_alias/"+alias, nil, http.StatusOK)
	if err != nil {
		return "", err
	}
	var payload map[string]struct {
		Aliases map[string]json.RawMessage `json:"aliases"`
	}
	if json.Unmarshal(response, &payload) != nil || len(payload) != 1 {
		return "", malformedFailure(OperationResolveAlias, ErrLifecycleRejected)
	}
	var index string
	for index = range payload {
	}
	value := payload[index]
	if !indexTargetPattern.MatchString(index) {
		return "", ErrUnsafeIndexTarget
	}
	if _, exists := value.Aliases[alias]; !exists {
		return "", malformedFailure(OperationResolveAlias, ErrLifecycleRejected)
	}
	return index, nil
}

// SwapAlias performs only the atomic alias mutation. It does not quiesce
// writers or verify generations; migration cutovers must use CutoverAlias.
// SwapAlias remains appropriate for bootstrap and externally fenced rollback.
func (client *Client) SwapAlias(ctx context.Context, tenant, alias, from, to string) error {
	if !validLifecycleTenant(tenant) || !indexTargetPattern.MatchString(alias) || !indexTargetPattern.MatchString(from) ||
		!indexTargetPattern.MatchString(to) || from == to || alias == from || alias == to {
		return ErrUnsafeIndexTarget
	}
	if err := client.authorizeLifecycle(ctx, tenant, alias, from, to); err != nil {
		return err
	}
	return client.withLifecycleMutation(ctx, LifecycleMutationRequest{
		Tenant: tenant, Operation: OperationSwapAlias, Resources: []string{alias, from, to},
	}, func(operationCtx context.Context) error {
		return client.swapAlias(operationCtx, alias, from, to)
	})
}

func (client *Client) swapAlias(ctx context.Context, alias, from, to string) error {
	body, _ := json.Marshal(map[string]any{"actions": []any{map[string]any{"remove": map[string]any{"index": from, "alias": alias, "must_exist": true}}, map[string]any{"add": map[string]any{"index": to, "alias": alias, "is_write_index": true}}}})
	response, err := client.executeMutation(ctx, OperationSwapAlias, http.MethodPost, "/_aliases", body, http.StatusOK)
	if err != nil {
		return err
	}
	var acknowledged struct {
		Acknowledged bool `json:"acknowledged"`
	}
	if json.Unmarshal(response, &acknowledged) != nil || !acknowledged.Acknowledged {
		return unknownMalformedFailure(OperationSwapAlias, ErrLifecycleRejected)
	}
	return nil
}

// AddAlias creates an authorized alias for a physical generation. It is used
// for initial bootstrap; later generation changes use atomic SwapAlias.
func (client *Client) AddAlias(ctx context.Context, tenant, alias, index string, write bool) error {
	if !validLifecycleTenant(tenant) || !validPhysicalName(alias) || !validPhysicalName(index) || alias == index {
		return ErrUnsafeIndexTarget
	}
	if err := client.authorizeLifecycle(ctx, tenant, alias, index); err != nil {
		return err
	}
	return client.withLifecycleMutation(ctx, LifecycleMutationRequest{
		Tenant: tenant, Operation: OperationSwapAlias, Resources: []string{alias, index},
	}, func(operationCtx context.Context) error {
		body, _ := json.Marshal(map[string]any{"actions": []any{map[string]any{"add": map[string]any{"index": index, "alias": alias, "is_write_index": write}}}})
		response, err := client.executeMutation(operationCtx, OperationSwapAlias, http.MethodPost, "/_aliases", body, http.StatusOK)
		if err != nil {
			return err
		}
		return requireAcknowledged(OperationSwapAlias, response)
	})
}

// DeleteIndex refuses direct deletion with ErrLifecycleCleanupGuardRequired after
// basic identity validation. Use CleanupIndex with the complete migration binding.
func (client *Client) DeleteIndex(ctx context.Context, tenant, index string) error {
	if !validLifecycleTenant(tenant) || !indexTargetPattern.MatchString(index) {
		return ErrUnsafeIndexTarget
	}
	return ErrLifecycleCleanupGuardRequired
}

// CleanupIndex performs irreversible physical deletion only while the
// application-owned final eligibility guard proves the complete migration
// binding and keeps its durable exclusion active.
func (client *Client) CleanupIndex(ctx context.Context, request search.LifecycleCleanupRequest) error {
	if request.MigrationID == "" || len(request.MigrationID) > search.DefaultLimits().MaxIDBytes ||
		!utf8.ValidString(request.MigrationID) || !validLifecycleTenant(request.Tenant) ||
		!indexTargetPattern.MatchString(request.Alias) || !indexTargetPattern.MatchString(request.ActiveIndex) ||
		!indexTargetPattern.MatchString(request.InactiveIndex) || request.ActiveIndex == request.InactiveIndex ||
		request.Alias == request.ActiveIndex || request.Alias == request.InactiveIndex ||
		!indexFingerprintPattern.MatchString(request.ActiveFingerprint) || !indexFingerprintPattern.MatchString(request.InactiveFingerprint) {
		return ErrUnsafeIndexTarget
	}
	if err := client.authorizeLifecycle(ctx, request.Tenant, request.Alias, request.ActiveIndex, request.InactiveIndex); err != nil {
		return err
	}
	if client.lifecycle.CleanupGuard == nil {
		return ErrLifecycleCleanupGuardRequired
	}
	return client.withLifecycleMutation(ctx, LifecycleMutationRequest{
		Tenant: request.Tenant, Operation: OperationDeleteIndex,
		Resources: []string{request.Alias, request.ActiveIndex, request.InactiveIndex},
	}, func(operationCtx context.Context) error {
		return client.cleanupIndexEligible(operationCtx, request)
	})
}

// Check callback cardinality/completion without claiming that these process-
// local flags establish eligibility or exclusion in an application store.
func (client *Client) cleanupIndexEligible(ctx context.Context, request search.LifecycleCleanupRequest) error {
	var calls atomic.Uint32
	var active atomic.Bool
	var completed atomic.Bool
	results := make(chan error, 1)
	operationCtx, cancelOperation := context.WithCancel(ctx)
	defer cancelOperation()
	active.Store(true)
	guardErr := client.lifecycle.CleanupGuard.WithCleanupEligible(operationCtx, request, func() error {
		if calls.Add(1) != 1 || !active.Load() {
			return lifecycleCleanupGuardFailure(nil)
		}
		operationErr := client.deleteIndex(operationCtx, request.InactiveIndex)
		completed.Store(true)
		results <- operationErr
		return operationErr
	})
	completedAtGuardReturn := completed.Load()
	active.Store(false)
	cancelOperation()
	if calls.Load() == 0 {
		return lifecycleCleanupGuardFailure(nil)
	}
	operationErr := <-results
	if calls.Load() != 1 {
		return lifecycleCleanupGuardFailure(operationErr)
	}
	if !completedAtGuardReturn {
		return lifecycleCleanupGuardUnknownFailure()
	}
	if operationErr != nil {
		if guardErr != nil && !errors.Is(guardErr, operationErr) {
			return lifecycleCleanupGuardFailure(operationErr)
		}
		return operationErr
	}
	if guardErr != nil {
		return lifecycleCleanupGuardFailure(nil)
	}
	return nil
}

func (client *Client) deleteIndex(ctx context.Context, index string) error {
	response, err := client.executeMutation(ctx, OperationDeleteIndex, http.MethodDelete, "/"+index, nil, http.StatusOK)
	if err != nil {
		return err
	}
	var acknowledged struct {
		Acknowledged bool `json:"acknowledged"`
	}
	if json.Unmarshal(response, &acknowledged) != nil || !acknowledged.Acknowledged {
		return unknownMalformedFailure(OperationDeleteIndex, ErrLifecycleRejected)
	}
	return nil
}

func lifecycleCleanupGuardFailure(operationErr error) *Failure {
	cause := error(ErrLifecycleCleanupGuardRejected)
	if operationErr != nil {
		cause = errors.Join(operationErr, ErrLifecycleCleanupGuardRejected)
	}
	return &Failure{
		Operation: OperationDeleteIndex, Category: FailureRejected,
		OutcomeKnown: operationErr == nil, cause: cause,
	}
}

func lifecycleCleanupGuardUnknownFailure() *Failure {
	return &Failure{
		Operation: OperationDeleteIndex, Category: FailureRejected,
		OutcomeKnown: false, cause: ErrLifecycleCleanupGuardRejected,
	}
}

func (client *Client) withLifecycleMutation(ctx context.Context, request LifecycleMutationRequest, operation func(context.Context) error) error {
	if client.lifecycle.MutationGuard == nil {
		return ErrLifecycleMutationGuardRequired
	}
	request.Resources = append([]string(nil), request.Resources...)
	operationCtx, cancelOperation := context.WithCancel(ctx)
	defer cancelOperation()
	var calls atomic.Uint32
	var active atomic.Bool
	var completed atomic.Bool
	results := make(chan error, 1)
	active.Store(true)
	guardErr := client.lifecycle.MutationGuard.WithLifecycleMutation(operationCtx, request, func() error {
		if calls.Add(1) != 1 || !active.Load() {
			return lifecycleMutationGuardFailure(request.Operation, true, nil)
		}
		operationErr := operation(operationCtx)
		completed.Store(true)
		results <- operationErr
		return operationErr
	})
	completedAtGuardReturn := completed.Load()
	active.Store(false)
	cancelOperation()
	// Pre-callback context wrapping remains observable through Failure.Unwrap;
	// application guards must keep that retained chain free of private details.
	if calls.Load() == 0 && (errors.Is(guardErr, context.Canceled) || errors.Is(guardErr, context.DeadlineExceeded)) {
		return cancelledFailure(request.Operation, guardErr)
	}
	if calls.Load() == 0 {
		return lifecycleMutationGuardFailure(request.Operation, true, nil)
	}
	operationErr := <-results
	if calls.Load() != 1 {
		return lifecycleMutationGuardFailure(request.Operation, false, operationErr)
	}
	if !completedAtGuardReturn {
		return lifecycleMutationGuardFailure(request.Operation, false, operationErr)
	}
	if operationErr != nil {
		if guardErr != nil && !errors.Is(guardErr, operationErr) {
			return lifecycleMutationGuardFailure(request.Operation, false, operationErr)
		}
		return operationErr
	}
	if guardErr != nil {
		return lifecycleMutationGuardFailure(request.Operation, true, nil)
	}
	return nil
}

func lifecycleMutationGuardFailure(operation Operation, outcomeKnown bool, operationErr error) *Failure {
	cause := error(ErrLifecycleMutationGuardRejected)
	if operationErr != nil {
		cause = errors.Join(operationErr, ErrLifecycleMutationGuardRejected)
	}
	return &Failure{
		Operation: operation, Category: FailureRejected,
		OutcomeKnown: outcomeKnown, cause: cause,
	}
}

func (client *Client) authorizeLifecycle(ctx context.Context, tenant string, resources ...string) error {
	if ctx == nil {
		return ErrContextRequired
	}
	if err := ctx.Err(); err != nil {
		return cancelledFailure(OperationLifecycle, err)
	}
	if client.lifecycle == nil {
		return ErrLifecycleDisabled
	}
	if err := client.lifecycle.Authorizer.Authorize(ctx, tenant, append([]string(nil), resources...)); err != nil {
		return sanitizedCallbackFailure(OperationLifecycle, ErrLifecycleDenied, err)
	}
	if err := ctx.Err(); err != nil {
		return cancelledFailure(OperationLifecycle, err)
	}
	return nil
}

func containsUnsafePath(value string) bool {
	for _, character := range value {
		if character == '/' || character == '\\' || character == '?' || character == '#' || character == '%' || character == '\x00' || character == '\r' || character == '\n' {
			return true
		}
	}
	return false
}

func validLifecycleTenant(tenant string) bool {
	return tenant != "" && len(tenant) <= search.DefaultLimits().MaxTenantBytes && utf8.ValidString(tenant)
}
