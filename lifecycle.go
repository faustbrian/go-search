package search

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"sync/atomic"
	"unicode/utf8"
)

var (
	// ErrInvalidMigrator requires nonnil dependencies and a store implementing
	// MigrationCoordinator.
	ErrInvalidMigrator = errors.New("search: lifecycle dependencies are required")
	// ErrInvalidMigrationPlan rejects invalid or unbound generation identities and per-call
	// step limits.
	ErrInvalidMigrationPlan = errors.New("search: invalid migration plan")
	// ErrMigrationNotFound is the store sentinel for absence, which Run treats as pending work.
	ErrMigrationNotFound = errors.New("search: migration state not found")
	// ErrMigrationIncomplete indicates exhausted per-call reindex steps with a resumable
	// checkpoint.
	ErrMigrationIncomplete = errors.New("search: migration remains resumable")
	// ErrMigrationPlanChanged rejects persisted identity/fingerprint mismatch; it does not
	// reset state.
	ErrMigrationPlanChanged = errors.New("search: migration plan changed after execution began")
	// ErrMigrationVerification rejects unverified or nonzero-drift backend attestation.
	ErrMigrationVerification = errors.New("search: migration verification failed")
	// ErrMigrationRecovery refuses ambiguous intent phases until application-owned recovery
	// repairs the checkpoint.
	ErrMigrationRecovery = errors.New("search: migration requires application-owned recovery before retry")
	// ErrAliasChanged refuses alias state that no longer matches the expected physical generation.
	ErrAliasChanged = errors.New("search: alias no longer identifies the expected generation")
	// ErrInvalidMigrationPhase rejects a checkpoint phase or cursor invalid for the requested
	// lifecycle action.
	ErrInvalidMigrationPhase = errors.New("search: operation is invalid for migration phase")
	// ErrMigrationCoordination identifies omitted, repeated, or incomplete coordinator
	// callback execution.
	ErrMigrationCoordination = errors.New("search: migration coordination contract was violated")
)

// MigrationPhase records a durable lifecycle checkpoint. Dispatch-intent phases require
// application recovery after ambiguity.
type MigrationPhase string

const (
	// MigrationPending has not begun creation.
	MigrationPending MigrationPhase = "pending"
	// MigrationCreating checkpoints create intent; restart requires recovery.
	MigrationCreating MigrationPhase = "creating"
	// MigrationCreated records acknowledged target creation.
	MigrationCreated MigrationPhase = "created"
	// MigrationDispatching checkpoints task submission intent; restart requires recovery.
	MigrationDispatching MigrationPhase = "reindex_dispatching"
	// MigrationReindexing records a durable task continuation.
	MigrationReindexing MigrationPhase = "reindexing"
	// MigrationReindexed records backend task completion, not semantic verification.
	MigrationReindexed MigrationPhase = "reindexed"
	// MigrationVerified records verification before fresh fenced cutover.
	MigrationVerified MigrationPhase = "verified"
	// MigrationComplete records successful cutover checkpoint.
	MigrationComplete MigrationPhase = "complete"
	// MigrationRolledBack records successful reverse cutover.
	MigrationRolledBack MigrationPhase = "rolled_back"
	// MigrationCleaning checkpoints deletion intent; restart requires recovery.
	MigrationCleaning MigrationPhase = "cleaning"
	// MigrationCleaned records deletion completion and is terminal.
	MigrationCleaned MigrationPhase = "cleaned"
)

// LifecycleOperation identifies the separately authorized/observed migration step.
type LifecycleOperation string

const (
	// LifecycleCreate authorizes/observes physical generation creation.
	LifecycleCreate LifecycleOperation = "create"
	// LifecycleReindex authorizes/observes task dispatch or polling.
	LifecycleReindex LifecycleOperation = "reindex"
	// LifecycleVerify authorizes/observes semantic verification.
	LifecycleVerify LifecycleOperation = "verify"
	// LifecycleCutover authorizes/observes forward alias movement.
	LifecycleCutover LifecycleOperation = "cutover"
	// LifecycleRollback authorizes/observes reverse verified alias movement.
	LifecycleRollback LifecycleOperation = "rollback"
	// LifecycleCleanup authorizes/observes irreversible inactive-generation deletion.
	LifecycleCleanup LifecycleOperation = "cleanup"
)

// MigrationPlan binds immutable generation identities. MaxReindexSteps is a per-call
// polling budget, excluded from its identity fingerprint.
type MigrationPlan struct {
	// ID is the stable durable migration identity.
	ID string
	// Tenant is the application scope presented to dependencies.
	Tenant string
	// Alias is distinct from both physical generations.
	Alias string
	// SourceIndex is the retained prior physical generation.
	SourceIndex string
	// SourceFingerprint is the lowercase SHA-256 definition identity used for rollback.
	SourceFingerprint string
	// Target is a validated immutable target definition.
	Target IndexDefinition
	// MaxReindexSteps bounds polls per Run, not total task lifetime; it may change between
	// resumptions.
	MaxReindexSteps int
}

// VerificationReport is backend evidence; counts alone do not prove semantic equivalence.
type VerificationReport struct {
	// Verified is the backend semantic attestation; the core also requires zero Drift.
	Verified bool
	// SourceCount is observed source cardinality.
	SourceCount uint64
	// TargetCount is observed target cardinality.
	TargetCount uint64
	// Drift is backend-reported semantic difference count.
	Drift uint64
}

// MigrationState is application-persisted workflow state, not an atomic transaction with
// backend effects.
type MigrationState struct {
	// ID binds the checkpoint to the plan.
	ID string
	// PlanFingerprint binds generation identities, excluding the per-run poll budget.
	PlanFingerprint string
	// Phase is the last saved workflow checkpoint.
	Phase MigrationPhase
	// ReindexCursor is sensitive opaque backend continuation data owned by the checkpoint store.
	ReindexCursor string
	// Verification retains the last semantic verification report.
	Verification VerificationReport
}

// LifecycleIntent describes one application authorization decision before a backend step.
type LifecycleIntent struct {
	// MigrationID identifies the workflow; Tenant and Resource are sensitive application
	// authorization scope.
	MigrationID, Tenant, Resource string
	// Operation identifies the requested step.
	Operation LifecycleOperation
}

// LifecycleEvent is emitted after saving a checkpoint; observer failure does not roll back
// that saved state.
type LifecycleEvent struct {
	// MigrationID identifies the workflow; Tenant and Resource are sensitive checkpoint scope,
	// not safe metric labels.
	MigrationID, Tenant, Resource string
	// Operation identifies the saved step.
	Operation LifecycleOperation
	// Phase identifies the saved checkpoint.
	Phase MigrationPhase
}

// LifecycleCleanupRequest binds irreversible deletion to the exact migration,
// active generation, inactive generation, and immutable definitions whose
// final eligibility must be proven under one durable exclusion boundary.
type LifecycleCleanupRequest struct {
	// MigrationID is the durable workflow identity.
	MigrationID string
	// Tenant scopes application policy; Alias must identify the active generation.
	Tenant, Alias string
	// ActiveIndex and ActiveFingerprint identify the retained live generation.
	ActiveIndex, ActiveFingerprint string
	// InactiveIndex and InactiveFingerprint identify the exact generation eligible for
	// irreversible deletion.
	InactiveIndex, InactiveFingerprint string
}

// LifecycleBackend implements the backend steps, including fresh semantic attestation and
// application-fenced mutations.
// The core delegates these obligations; it cannot independently prove durable exclusion or
// backend completion.
type LifecycleBackend interface {
	// CreateIndex creates the authorized definition; ambiguous receipt requires recovery
	// rather than blind resubmission.
	CreateIndex(context.Context, string, IndexDefinition) error
	// Reindex submits or polls an explicit task; a nonfinal result must return a durable cursor.
	Reindex(context.Context, string, string, string, string) (cursor string, done bool, err error)
	// VerifyIndex compares complete bounded generation semantics and attests the expected live
	// definition.
	VerifyIndex(context.Context, string, string, string, string) (VerificationReport, error)
	// ResolveAlias returns the single physical generation currently selected by the logical alias.
	ResolveAlias(context.Context, string, string) (string, error)
	// CutoverAlias must freshly verify under continuous application write quiescence and
	// atomically move the alias.
	CutoverAlias(context.Context, string, string, string, string, string) (VerificationReport, error)
	// CleanupIndex must delete only while the complete durable final-eligibility fence remains
	// held.
	CleanupIndex(context.Context, LifecycleCleanupRequest) error
}

// MigrationStore owns durable checkpoints and must also implement MigrationCoordinator for
// NewMigrator.
type MigrationStore interface {
	// Load retrieves one checkpoint or reports ErrMigrationNotFound; the store owns durable
	// isolation.
	Load(context.Context, string) (MigrationState, error)
	// Save persists the checkpoint; failures can follow already completed backend effects.
	Save(context.Context, MigrationState) error
}

// MigrationCoordinator provides one durable exclusive execution boundary for
// a migration ID across all application instances. WithMigration must invoke
// operation synchronously exactly once and keep exclusivity until it returns.
// The boundary intentionally spans backend I/O so cleanup, rollback, cutover,
// and reindex dispatch cannot race one another.
type MigrationCoordinator interface {
	// WithMigration invokes the operation synchronously exactly once under durable
	// cross-instance exclusion until return.
	WithMigration(context.Context, string, func(context.Context) error) error
}

// LifecycleAuthorizer approves a migration step synchronously; policy errors are returned
// without core redaction.
type LifecycleAuthorizer interface {
	// Authorize approves the scoped step or returns an error before that step is dispatched.
	Authorize(context.Context, LifecycleIntent) error
}

// LifecycleObserver records saved checkpoints synchronously; it may receive sensitive
// tenant/resource identities.
type LifecycleObserver interface {
	// Record observes an already saved checkpoint; failure aborts Run but cannot undo the save.
	Record(context.Context, LifecycleEvent) error
}

// Migrator borrows backend, store, authorization, and observer implementations.
// Calls are synchronous; durable coordination, panic safety, cancellation, and shutdown
// remain dependency/application responsibilities.
type Migrator struct {
	backend     LifecycleBackend
	store       MigrationStore
	coordinator MigrationCoordinator
	authorizer  LifecycleAuthorizer
	observer    LifecycleObserver
}

// NewMigrator borrows nonnil dependencies and requires the store to implement
// MigrationCoordinator. It does not qualify their durability, typed-nil safety,
// callback cooperation, authorization correctness, or backend atomicity.
func NewMigrator(backend LifecycleBackend, store MigrationStore, authorizer LifecycleAuthorizer, observer LifecycleObserver) (*Migrator, error) {
	if backend == nil || store == nil || authorizer == nil || observer == nil {
		return nil, ErrInvalidMigrator
	}
	coordinator, ok := store.(MigrationCoordinator)
	if !ok {
		return nil, ErrInvalidMigrator
	}
	return &Migrator{backend: backend, store: store, coordinator: coordinator, authorizer: authorizer, observer: observer}, nil
}

// Run resumes an explicit create, reindex, verify, and atomic alias-cutover
// workflow under store-owned durable exclusion. Intent checkpoints precede
// ambiguous create/reindex/delete effects; saved completion and observer errors
// are not transactions with backend I/O. Recovery is application-owned.
func (m *Migrator) Run(ctx context.Context, plan MigrationPlan) (MigrationState, error) {
	return coordinateMigration(ctx, m.coordinator, plan.ID, func(operationCtx context.Context) (MigrationState, error) {
		return m.run(operationCtx, plan)
	})
}

func (m *Migrator) run(ctx context.Context, plan MigrationPlan) (MigrationState, error) {
	fingerprint, err := validateMigrationPlan(plan)
	if err != nil {
		return MigrationState{}, err
	}
	state, err := m.store.Load(ctx, plan.ID)
	if errors.Is(err, ErrMigrationNotFound) {
		state = MigrationState{ID: plan.ID, PlanFingerprint: fingerprint, Phase: MigrationPending}
	} else if err != nil {
		return MigrationState{}, err
	} else if state.ID != plan.ID || state.PlanFingerprint != fingerprint {
		return state, ErrMigrationPlanChanged
	}
	if len(state.ReindexCursor) > DefaultLimits().MaxQueryBytes {
		return state, ErrInvalidMigrationPhase
	}
	if state.Phase == MigrationComplete {
		return state, nil
	}
	if state.Phase == MigrationCreating || state.Phase == MigrationDispatching || state.Phase == MigrationCleaning {
		return state, ErrMigrationRecovery
	}
	if state.Phase == MigrationRolledBack || state.Phase == MigrationCleaned {
		return state, ErrInvalidMigrationPhase
	}
	switch state.Phase {
	case MigrationPending, MigrationCreated, MigrationReindexing, MigrationReindexed, MigrationVerified:
	default:
		return state, ErrInvalidMigrationPhase
	}

	if state.Phase == MigrationPending {
		if err := m.authorize(ctx, plan, LifecycleCreate, plan.Target.Name()); err != nil {
			return state, err
		}
		state.Phase = MigrationCreating
		if err := m.checkpoint(ctx, plan, state, LifecycleCreate, plan.Target.Name()); err != nil {
			return state, err
		}
		if err := m.backend.CreateIndex(ctx, plan.Tenant, plan.Target); err != nil {
			return state, err
		}
		state.Phase = MigrationCreated
		if err := m.checkpoint(ctx, plan, state, LifecycleCreate, plan.Target.Name()); err != nil {
			return state, err
		}
	}
	reindexAuthorized := false
	if state.Phase == MigrationCreated {
		if err := m.authorize(ctx, plan, LifecycleReindex, plan.SourceIndex); err != nil {
			return state, err
		}
		state.Phase = MigrationDispatching
		if err := m.checkpoint(ctx, plan, state, LifecycleReindex, plan.SourceIndex); err != nil {
			return state, err
		}
		reindexAuthorized = true
	}
	if state.Phase == MigrationDispatching || state.Phase == MigrationReindexing {
		if state.Phase == MigrationReindexing && state.ReindexCursor == "" {
			return state, ErrMigrationRecovery
		}
		for range plan.MaxReindexSteps {
			if !reindexAuthorized {
				if err := m.authorize(ctx, plan, LifecycleReindex, plan.SourceIndex); err != nil {
					return state, err
				}
			}
			reindexAuthorized = false
			cursor, done, reindexErr := m.backend.Reindex(ctx, plan.Tenant, plan.SourceIndex, plan.Target.Name(), state.ReindexCursor)
			if reindexErr != nil {
				return state, reindexErr
			}
			if len(cursor) > DefaultLimits().MaxQueryBytes || !done && cursor == "" {
				return state, ErrMigrationRecovery
			}
			state.ReindexCursor = cursor
			if done {
				state.Phase = MigrationReindexed
			} else {
				state.Phase = MigrationReindexing
			}
			if err := m.checkpoint(ctx, plan, state, LifecycleReindex, plan.SourceIndex); err != nil {
				return state, err
			}
			if done {
				break
			}
		}
		if state.Phase != MigrationReindexed {
			return state, ErrMigrationIncomplete
		}
	}
	if state.Phase == MigrationReindexed {
		if err := m.authorize(ctx, plan, LifecycleVerify, plan.Target.Name()); err != nil {
			return state, err
		}
		report, verifyErr := m.backend.VerifyIndex(ctx, plan.Tenant, plan.SourceIndex, plan.Target.Name(), plan.Target.Fingerprint())
		if verifyErr != nil {
			return state, verifyErr
		}
		if !report.Verified || report.Drift != 0 {
			return state, ErrMigrationVerification
		}
		state.Verification, state.Phase = report, MigrationVerified
		if err := m.checkpoint(ctx, plan, state, LifecycleVerify, plan.Target.Name()); err != nil {
			return state, err
		}
	}
	if state.Phase == MigrationVerified {
		if err := m.authorize(ctx, plan, LifecycleCutover, plan.Alias); err != nil {
			return state, err
		}
		current, resolveErr := m.backend.ResolveAlias(ctx, plan.Tenant, plan.Alias)
		if resolveErr != nil {
			return state, resolveErr
		}
		if current != plan.SourceIndex {
			return state, ErrAliasChanged
		}
		fresh, cutoverErr := m.backend.CutoverAlias(ctx, plan.Tenant, plan.Alias, plan.SourceIndex, plan.Target.Name(), plan.Target.Fingerprint())
		if cutoverErr != nil {
			return state, cutoverErr
		}
		if !fresh.Verified || fresh.Drift != 0 {
			return state, ErrMigrationVerification
		}
		state.Verification = fresh
		state.Phase = MigrationComplete
		if err := m.checkpoint(ctx, plan, state, LifecycleCutover, plan.Alias); err != nil {
			return state, err
		}
	}
	return state, nil
}

// Rollback atomically restores the prior alias generation. It does not delete
// either index.
func (m *Migrator) Rollback(ctx context.Context, plan MigrationPlan) (MigrationState, error) {
	return coordinateMigration(ctx, m.coordinator, plan.ID, func(operationCtx context.Context) (MigrationState, error) {
		return m.rollback(operationCtx, plan)
	})
}

func (m *Migrator) rollback(ctx context.Context, plan MigrationPlan) (MigrationState, error) {
	state, err := m.loadBound(ctx, plan)
	if err != nil {
		return state, err
	}
	if state.Phase == MigrationRolledBack {
		return state, nil
	}
	if state.Phase != MigrationComplete {
		return state, ErrInvalidMigrationPhase
	}
	if err := m.authorize(ctx, plan, LifecycleRollback, plan.Alias); err != nil {
		return state, err
	}
	current, err := m.backend.ResolveAlias(ctx, plan.Tenant, plan.Alias)
	if err != nil {
		return state, err
	}
	if current != plan.Target.Name() {
		return state, ErrAliasChanged
	}
	fresh, cutoverErr := m.backend.CutoverAlias(ctx, plan.Tenant, plan.Alias, plan.Target.Name(), plan.SourceIndex, plan.SourceFingerprint)
	if cutoverErr != nil {
		return state, cutoverErr
	}
	if !fresh.Verified || fresh.Drift != 0 {
		return state, ErrMigrationVerification
	}
	state.Verification = fresh
	state.Phase = MigrationRolledBack
	if err := m.checkpoint(ctx, plan, state, LifecycleRollback, plan.Alias); err != nil {
		return state, err
	}
	return state, nil
}

// Cleanup deletes only the inactive generation after successful completion or
// rollback and therefore remains separately authorized.
func (m *Migrator) Cleanup(ctx context.Context, plan MigrationPlan) (MigrationState, error) {
	return coordinateMigration(ctx, m.coordinator, plan.ID, func(operationCtx context.Context) (MigrationState, error) {
		return m.cleanup(operationCtx, plan)
	})
}

func (m *Migrator) cleanup(ctx context.Context, plan MigrationPlan) (MigrationState, error) {
	state, err := m.loadBound(ctx, plan)
	if err != nil {
		return state, err
	}
	if state.Phase == MigrationCleaned {
		return state, nil
	}
	if state.Phase == MigrationCleaning {
		return state, ErrMigrationRecovery
	}
	var active, inactive string
	switch state.Phase {
	case MigrationComplete:
		active = plan.Target.Name()
		inactive = plan.SourceIndex
	case MigrationRolledBack:
		active = plan.SourceIndex
		inactive = plan.Target.Name()
	default:
		return state, ErrInvalidMigrationPhase
	}
	if err := m.authorize(ctx, plan, LifecycleCleanup, inactive); err != nil {
		return state, err
	}
	current, err := m.backend.ResolveAlias(ctx, plan.Tenant, plan.Alias)
	if err != nil {
		return state, err
	}
	if current != active {
		return state, ErrAliasChanged
	}
	state.Phase = MigrationCleaning
	if err := m.checkpoint(ctx, plan, state, LifecycleCleanup, inactive); err != nil {
		return state, err
	}
	activeFingerprint, inactiveFingerprint := plan.Target.Fingerprint(), plan.SourceFingerprint
	if active == plan.SourceIndex {
		activeFingerprint, inactiveFingerprint = plan.SourceFingerprint, plan.Target.Fingerprint()
	}
	if err := m.backend.CleanupIndex(ctx, LifecycleCleanupRequest{
		MigrationID: plan.ID, Tenant: plan.Tenant, Alias: plan.Alias,
		ActiveIndex: active, ActiveFingerprint: activeFingerprint,
		InactiveIndex: inactive, InactiveFingerprint: inactiveFingerprint,
	}); err != nil {
		return state, err
	}
	state.Phase = MigrationCleaned
	if err := m.checkpoint(ctx, plan, state, LifecycleCleanup, inactive); err != nil {
		return state, err
	}
	return state, nil
}

// Detect omitted/repeated/late completion, but do not mistake callback-count
// checks for proof that a coordinator holds a durable cross-instance fence.
func coordinateMigration[T any](ctx context.Context, coordinator MigrationCoordinator, id string, operation func(context.Context) (T, error)) (T, error) {
	type migrationOutcome struct {
		result T
		err    error
	}
	var zero T
	var calls atomic.Uint32
	var phase atomic.Uint32
	outcomes := make(chan migrationOutcome, 1)
	operationCtx, cancelOperation := context.WithCancel(ctx)
	defer cancelOperation()
	coordinationErr := coordinator.WithMigration(operationCtx, id, func(callbackCtx context.Context) error {
		if calls.Add(1) != 1 {
			return ErrMigrationCoordination
		}
		if phase.Load() != 0 {
			return ErrMigrationCoordination
		}
		result, operationErr := operation(callbackCtx)
		if !phase.CompareAndSwap(0, 1) {
			return ErrMigrationCoordination
		}
		outcomes <- migrationOutcome{result: result, err: operationErr}
		return operationErr
	})
	phase.Swap(2)
	cancelOperation()
	if calls.Load() == 0 && coordinationErr != nil {
		return zero, coordinationErr
	}
	if calls.Load() != 1 {
		return zero, ErrMigrationCoordination
	}
	var outcome migrationOutcome
	select {
	case outcome = <-outcomes:
	default:
		return zero, ErrMigrationCoordination
	}
	if coordinationErr != nil {
		return outcome.result, coordinationErr
	}
	return outcome.result, outcome.err
}

func (m *Migrator) loadBound(ctx context.Context, plan MigrationPlan) (MigrationState, error) {
	fingerprint, err := validateMigrationPlan(plan)
	if err != nil {
		return MigrationState{}, err
	}
	state, err := m.store.Load(ctx, plan.ID)
	if err != nil {
		return state, err
	}
	if state.ID != plan.ID || state.PlanFingerprint != fingerprint {
		return state, ErrMigrationPlanChanged
	}
	return state, nil
}
func (m *Migrator) authorize(ctx context.Context, plan MigrationPlan, operation LifecycleOperation, resource string) error {
	return m.authorizer.Authorize(ctx, LifecycleIntent{MigrationID: plan.ID, Tenant: plan.Tenant, Resource: resource, Operation: operation})
}
func (m *Migrator) checkpoint(ctx context.Context, plan MigrationPlan, state MigrationState, operation LifecycleOperation, resource string) error {
	if err := m.store.Save(ctx, state); err != nil {
		return err
	}
	return m.observer.Record(ctx, LifecycleEvent{MigrationID: plan.ID, Tenant: plan.Tenant, Resource: resource, Operation: operation, Phase: state.Phase})
}
func validateMigrationPlan(plan MigrationPlan) (string, error) {
	target := plan.Target.Name()
	limits := DefaultLimits()
	if plan.ID == "" || len(plan.ID) > limits.MaxIDBytes || !utf8.ValidString(plan.ID) ||
		plan.Tenant == "" || len(plan.Tenant) > limits.MaxTenantBytes || !utf8.ValidString(plan.Tenant) ||
		plan.Alias == "" || plan.SourceIndex == "" || !validDefinitionFingerprint(plan.SourceFingerprint) ||
		target == "" || !validDefinitionFingerprint(plan.Target.Fingerprint()) ||
		plan.MaxReindexSteps <= 0 || plan.MaxReindexSteps > limits.MaxPages ||
		!validIndexName(plan.SourceIndex) || !validIndexName(plan.Alias) ||
		plan.SourceIndex == target || plan.Alias == plan.SourceIndex || plan.Alias == target {
		return "", ErrInvalidMigrationPlan
	}
	sum := sha256.Sum256([]byte(fmt.Sprintf("%s\x00%s\x00%s\x00%s\x00%s\x00%s\x00%s", plan.ID, plan.Tenant, plan.Alias, plan.SourceIndex, plan.SourceFingerprint, target, plan.Target.Fingerprint())))
	return hex.EncodeToString(sum[:]), nil
}

func validDefinitionFingerprint(value string) bool {
	if len(value) != sha256.Size*2 {
		return false
	}
	decoded, _ := hex.DecodeString(value)
	return hex.EncodeToString(decoded) == value
}
