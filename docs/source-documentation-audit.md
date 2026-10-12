# Source documentation audit

## Outcome, owner, consumers, risks and smallest checks

Base 1fc8f9829b650419b27f4e8c170d0d1ffe27b1da; fresh-main detached task worktree. Tier A comments/docs only.
Root maintainers own bounded typed search/query/document/write/result and
source-owned lifecycle/rebuild/projection contracts. The OpenSearch adapter owns
explicit backend mapping, discovery, bounded response decode, write attribution,
PIT/cursor cleanup and safety checks. Applications own source-of-truth data,
authentication, tenant/storage authorization, callbacks/SDK/client lifetime,
refresh/retry policy and incident/recovery decisions. searchtest owns deterministic
fixtures/conformance helpers, not an independently qualified backend.

Audit all 105 tracked Go files, two catalog modules/three packages and the tracked
frozen compatibility fixture source path. Named risks: exact-number/Unicode/JSON admission,
query recursion/order/fingerprint ownership, signed cursor scope/key/time limits,
backend capabilities versus unsupported semantics, refresh/unknown/partial writes,
local tracked snapshot/PIT ownership, cancellation/clock/callback liveness,
source-version fencing/reconciliation/checkpoints and metadata privacy.

Smallest checks: source-derived public/member/header contracts and private
invariants; changed formatting; every tracked file's exact AST/noncomment tokens/
literals/directives and reported grammar-optional terminal semicolons; parsed HTML/native source renders;
configured native structural API baseline preservation; native source/local docs/
offline specification, links, preserved pinned decisions/history/fixtures;
default pinned full/changed/staged secret scans; ordinary complete final-diff
truth/usefulness inspection and explicit Intent commit. All Go/native workflows
wait for a coordinator grant; three owned caches/helpers/renders are removed on
success/failure. No unchanged examples/runtime/race/fuzz/bench/stress/leak/soak,
live OpenSearch/Docker/cluster/consumer/conformance/interoperability/online/release
campaigns. Existing docs targets run tests, so choose actual source/doc/link
checks rather than relabeling them a prose gate.

## Missing versus investigated

Initially missing: canonical root header and exported enums, fields,
methods/options/interface obligations. Retain meaningful existing constraints
rather than narrating syntax. Investigation needed: public mutable containers
versus copied values, backend limits/cancellation/privacy, exact cursor/version/
PIT ownership and cleanup, replay/partial outcomes, checkpoint persistence and
source-transaction boundaries. Unknown rationale stays explicit; no runtime or
normative selected-expectation repair is authorized.

## Reproducible complete topology and tracked-file inventory

Use git ls-files '*.go' '*go.mod' '*go.sum' '*AGENTS.md', modules.json/
packages.json, .golib.yaml, Make/verification/scripts/CI, declaration/marker and
test/helper/fuzz/benchmark/example scans. Both catalog module minima are Go
1.27.0; repository producer pins 1.27.2. Native API mode is apidiff for root and
OpenSearch; no prose snapshot generation is configured. Retain both baselines
and all frozen mixed-application inputs. Native producer is golib api check;
Make docs routes include runtime tests and are not selected for unchanged prose.

Tracked module/lock paths: `adapters/opensearch/go.mod`, `adapters/opensearch/go.sum`, `go.mod`

| Catalog module | Package | Import |
| --- | --- | --- |
| . | . | github.com/faustbrian/go-search |
| adapters/opensearch | . | github.com/faustbrian/go-search/adapters/opensearch |
| . | searchtest | github.com/faustbrian/go-search/searchtest |

| Tracked Go file | Package | Audit role/evidence |
| --- | --- | --- |
| `adapters/opensearch/aaa_mutation_lifecycle_test.go` | opensearch | Existing evidence: TestAAAACutoverCompletesHappyPathWithinBound, TestAAAACutoverRejectsOverlappingAliasGenerationsBeforeAuthorization, TestAAAACutoverAuthorizationSuccessRequiresGuard, TestAAAACleanupAcceptsMigrationIDAtByteLimit, TestAAAACleanupCompletesHappyPathWithinBound, TestAAAACleanupRejectsRepeatedGuardCallbackWithinBound, TestAAAALifecycleMutationRejectsRepeatedCallbackWithinBound, TestAAAAPointInTimeReleasePermitsIdentifierReacquisition, TestAAAAProjectionWildcardMatcherTracksEveryOrderedSegment, TestClientOwnsExplicitProxyURLConfiguration, TestInfoBoundsAndSanitizesReturnedIdentityFields, TestAAAMutationLifecycleVerificationAcceptsValidDriftBounds, TestLifecycleVerificationRejectsSameGenerationBeforeAuthorization, TestLifecycleCleanupRejectsAliasGenerationCollisionsBeforeAuthorization |
| `adapters/opensearch/aws_signer_cancellation_test.go` | opensearch_test | Existing evidence: TestOfficialAWSSignerCancellationReleasesAdmissionWithoutDispatch |
| `adapters/opensearch/benchmark_evidence_test.go` | opensearch_test | Existing evidence: TestBenchmarkEvidenceValidatorRequiresEverySuccessfulSample |
| `adapters/opensearch/benchmark_integration_test.go` | opensearch_test | Existing evidence: BenchmarkSharedSearchSemantics |
| `adapters/opensearch/benchmark_test.go` | opensearch_test | Existing evidence: BenchmarkSyntheticInfoTransport |
| `adapters/opensearch/bulk.go` | opensearch | Source API/private invariant |
| `adapters/opensearch/bulk_test.go` | opensearch_test | Existing evidence: TestBulkEncodesExternalVersionsAndPreservesPartialOutcomes, TestBulkClassifies429ClusterBlockAsNonRetryable, TestBulkAcceptsEverySupportedSuccessStatus, TestBulkClassifiesNotFoundOnlyForDelete, TestBulkDeleteRequiresExactNotFoundEvidence, TestBulkClassifiesVersionConflictOnlyFromExactBackendCode, TestBulkRejectsItemAttributedToAnotherPhysicalIndex, TestBulkReturnsPerItemUnknownOutcomesWhenTransportOutcomeIsAmbiguous, TestBulkRejectsEncodedBodyAboveConfiguredByteLimit, TestBulkRejectsMisattributedResponseActions, TestBulkRejectsInconsistentSuccessVersionAndErrorsFlag |
| `adapters/opensearch/cleanup_guard_integration_test.go` | opensearch_test | Existing evidence: TestRealOpenSearchCleanupGuardHoldsCompleteEligibilityAcrossDeletion |
| `adapters/opensearch/cleanup_integration_test.go` | opensearch_test | Existing evidence: TestRealOpenSearchConformanceCleanupSafeguards |
| `adapters/opensearch/cluster_integration_test.go` | opensearch_test | Existing evidence: TestRealOpenSearchMultiNodeRotation, TestRealOpenSearchEndpointFailoverBudget, TestRealOpenSearchCompleteOutageIsBounded, TestRealOpenSearchOutageRecoveryReconcilesUnknownWrites, TestRealOpenSearchRollingFixtureSeed, TestRealOpenSearchRollingFixtureVerify |
| `adapters/opensearch/compatibility.go` | opensearch | Source API/private invariant |
| `adapters/opensearch/compatibility_test.go` | opensearch | Existing evidence: TestSupportedOpenSearchVersionsIncludesCurrentRelease |
| `adapters/opensearch/config.go` | opensearch | Source API/private invariant |
| `adapters/opensearch/config_test.go` | opensearch_test | Existing evidence: TestClientUsesRotatedBasicCredentialsForEveryRequest, TestClientSelectsConfiguredNodesWithoutImplicitRetries, TestClientSignsEveryRequestAndRedactsSignerFailure, TestOfficialAWSSignerRetrievesRotatedCredentialsForEveryRequest, TestNewRejectsUnsafeOrAmbiguousTransportConfiguration, TestPointInTimeCapacityAcceptsDefaultAndExactMaximum, TestExplicitInsecureHTTPCannotCarryCredentials, TestBorrowedTransportRemainsCallerOwned, TestBorrowedHTTPTransportRemainsTheLiveCallerInstance, TestOwnedTransportIsUsedAndClosedExactlyOnce, TestInfoRejectsMalformedAndOversizedResponsesWithoutLeakingBodies, TestInfoRequiresContextAndClassifiesTransportAndStatusFailures |
| `adapters/opensearch/coverage_contract_test.go` | opensearch | Existing evidence: TestRemainingTransportOutcomes, TestRemainingDiscoveryHealthAndInfoOutcomes, TestRemainingLifecycleAndReconciliationOutcomes, TestRemainingResilienceOutcomes, TestRemainingSearchAndExecutionOutcomes |
| `adapters/opensearch/discovery.go` | opensearch | Source API/private invariant |
| `adapters/opensearch/discovery_test.go` | opensearch_test | Existing evidence: TestDiscoverReplacesSeedsOnlyWithExplicitlyTrustedDataNodes, TestDiscoverRejectsUntrustedOrMalformedTopologyWithoutChangingSeeds, TestDiscoverRequiresAnExplicitBoundedTrustPolicy |
| `adapters/opensearch/evidence_test.go` | opensearch_test | Existing evidence: TestEvidenceStressConcurrentInfoHasNoLostOutcomes, TestEvidenceLeakCancellationReleasesAdmission, TestEvidenceLeakMalformedResponseBodyIsClosed, TestEvidenceLeakRepeatedNetworkCallsBoundRetainedResources, TestEvidenceFaultMalformedBulkIsAmbiguousAndAttributed, TestEvidenceSoakSyntheticTransportBounded |
| `adapters/opensearch/example_test.go` | opensearch_test | Existing evidence: ExampleClient_Search |
| `adapters/opensearch/failure.go` | opensearch | Source API/private invariant |
| `adapters/opensearch/failure_test.go` | opensearch_test | Existing evidence: TestInfoClassifiesCancellationTransportAndOverload, TestInfoClassifiesMalformedErrorBodiesWithoutEchoingThem, TestCancellationClassificationDoesNotRetainPrivateTransportCauses |
| `adapters/opensearch/fault_integration_test.go` | opensearch_test | Existing evidence: TestRealOpenSearchConformanceLiveFaults |
| `adapters/opensearch/fuzz_test.go` | opensearch_test | Existing evidence: FuzzInfoResponseBoundary, FuzzSearchResponseBoundary, FuzzBulkResponseBoundary, FuzzLifecycleResponseBoundary |
| `adapters/opensearch/hardening_state_branches_test.go` | opensearch | Existing evidence: TestPointInTimeTrackerFailsClosedAcrossTerminalStates, TestPointInTimeTrackerRejectsInvalidRotationAndReapsExpiry, TestPointInTimeTrackerReapToleratesConcurrentRelease, TestCleanupIndexRejectsInvalidBindingAndMissingEligibilityGuard, TestCleanupIndexRejectsMissingRepeatedAndDivergentGuardOutcomes, TestLifecycleMutationGuardFailsClosedAcrossCallbackOutcomes, TestLifecycleAuthorizationObservesCancellationAfterPolicyCallback, TestSearchAuthorizationAndResolutionObserveCancellationAtBothBoundaries, TestProjectionPatternMatcherImplementsOnlyOpenSearchStarWildcards, TestSearchRejectsUnrepresentableCursorDeadlineBeforeCreatingPointInTime, TestSearchFailsClosedWhenPointInTimeTrackerClosesDuringCreation, TestSearchRejectsRotatedPointInTimeIdentifierOwnedByAnotherCursor, TestNilClientPointInTimeSnapshotIsEmpty |
| `adapters/opensearch/health.go` | opensearch | Source API/private invariant |
| `adapters/opensearch/health_test.go` | opensearch_test | Existing evidence: TestHealthAndCapacityPreserveOperationalSignals, TestHealthMarksRedOrIncompleteClustersUnready, TestHealthRejectsMissingOperationalFields |
| `adapters/opensearch/info.go` | opensearch | Source API/private invariant |
| `adapters/opensearch/integration_test.go` | opensearch_test | Existing evidence: TestRealOpenSearchConformanceSharedSemantics, TestRealOpenSearchConformanceGeneratedDSLDifferential, TestRealOpenSearchConformanceRebuildReconciliationRollbackAndConcurrentApplications, TestRealOpenSearchSnapshotRestore, TestRealOpenSearchBoundedLoad, TestRealOpenSearchConformance, TestRealOpenSearchDurableGuardSurvivesDeleteVersionGC, TestRealLifecycleVerifierUsesOnePointInTimeForBothGenerations, TestRealLifecycleVerifierCleansRotatedPointInTimeAfterScanFailure |
| `adapters/opensearch/internal_client_branches_test.go` | opensearch | Existing evidence: TestInternalReindexCursorRejectsAuthenticatedUnsafeBackendTask, TestInternalClientCapabilitiesWriteAndBulkFailures, TestInternalClientSearchFailureBranches, TestInternalLifecycleTemplateAndOperationalFailures, TestInternalHealthCapacityAndReconciliationFailures, TestInternalInfoTransportBodyFailures, TestInternalDiscoveryFailureBranches |
| `adapters/opensearch/internal_helpers_test.go` | opensearch | Existing evidence: TestInternalConfigAndTransportHelpers, TestInternalReindexCursorAndPaginationDefensiveBranches, TestInternalFailureWriteAndBulkClassifications, TestInternalSearchEncodingAndDecodingHelpers, TestInternalProjectionValidationCoversNestedObjectsAndArrays, TestInternalWriteAuthorizationChecksCancellationBeforeAndAfterGuard, TestInternalTelemetryResilienceAndMetricHelpers, TestInternalReadBoundedStatusAndPoolPerform, TestInternalExecuteAndDiscoveryHelpers |
| `adapters/opensearch/lifecycle.go` | opensearch | Source API/private invariant |
| `adapters/opensearch/lifecycle_cleanup_guard_test.go` | opensearch_test | Existing evidence: TestDirectIndexDeletionCannotBypassCleanupEligibilityGuard, TestCleanupIndexHoldsEligibilityGuardAcrossDeletion, TestAliasMutationCannotBypassActiveCleanupExclusion, TestSharedMutationGuardSerializesAliasMutationAfterCleanup, TestLifecycleMutationWaitsForAsynchronousCallbackBeforeRejectingGuard, TestCleanupIndexRejectsGuardThatReturnsBeforeDeletionCompletes |
| `adapters/opensearch/lifecycle_cursor.go` | opensearch | Source API/private invariant |
| `adapters/opensearch/lifecycle_cursor_internal_test.go` | opensearch | Existing evidence: TestReindexCursorCodecValidatesEachBindingBoundary, TestReindexCursorCodecRejectsEachEncryptedTokenDefect, TestReindexCursorCodecRejectsEachDecryptedEnvelopeDefect, TestReindexCursorCodecAcceptsExactEncodedSizeLimit, TestReindexCursorCodecRejectsExpiryOutsideUnixNanoRange |
| `adapters/opensearch/lifecycle_cursor_test.go` | opensearch_test | Existing evidence: TestReindexCursorIsEncryptedBoundAndExpiring, TestReindexRejectsStructurallyInvalidIncompleteTaskResponses, TestReindexRejectsInvalidStartedTaskIdentifiersAsMalformedUnknownOutcomes, TestReindexFailsBeforeDispatchWithoutCursorCodec, TestReindexRejectsIdenticalGenerationsBeforeDispatch, TestReindexRejectsOversizedTenantBeforeAuthorizationOrDispatch, TestReindexCursorCodecRejectsUnboundedConfigurationAndClock |
| `adapters/opensearch/lifecycle_cutover_test.go` | opensearch_test | Existing evidence: TestCutoverAliasHoldsApplicationFenceAcrossFinalVerificationAndSwap, TestCutoverAliasRequiresFenceAndVerifiedTargetBeforeSwap, TestLifecycleRejectsAliasAndPhysicalGenerationCollisionsBeforeDispatch, TestCutoverAliasHidesGuardFailureDetails, TestCutoverAliasRejectsLateGuardCallbackWithoutDispatch, TestCutoverAliasRejectsGuardReturningDuringCallbackWithoutSwap, TestCutoverAliasReportsGuardViolationWhenInternalCancellationStopsVerifier, TestCutoverAliasRejectsRepeatedGuardCallback, TestCutoverAliasConcurrentRepeatedGuardCallbackDoesNotBlockPrimary, TestCutoverAliasPreservesFirstFailureAndReportsPostFailureGuardViolation, TestCutoverAliasRedactsVerifierFailure, TestCutoverAliasPreservesOperationFailureWhenGuardReturnsNil, TestCutoverAliasHidesGuardFailureAfterCompletedCallback, TestCutoverAliasClassifiesGuardReturnDuringAcknowledgedAliasRequest, TestCutoverAliasPreservesAmbiguousSwapCancellationWhenGuardReturns, TestCutoverAliasPreservesUnverifiedFailureWhenGuardAlsoViolatesContract, TestCutoverAliasDeniesBeforeGuardOrDispatch |
| `adapters/opensearch/lifecycle_failure_test.go` | opensearch_test | Existing evidence: TestLifecycleMutationsReportAmbiguousTransportAndMalformedOutcomes, TestReadOnlyLifecycleFailuresKeepKnownOutcomes, TestReindexRejectsMalformedTaskCursorBeforeNetworkAccess |
| `adapters/opensearch/lifecycle_test.go` | opensearch_test | Existing evidence: TestLifecycleImplementsCreateResumableReindexVerifyCutoverAndCleanup, TestLifecycleRequiresAuthorizationBeforeNetworkAccess, TestAddAliasCreatesAnAuthorizedReadWriteBoundary |
| `adapters/opensearch/lifecycle_verifier_test.go` | opensearch_test | Existing evidence: TestVerifyIndexRequiresSemanticVerifierAfterCountPreflight, TestVerifyIndexUsesSemanticDriftWhenCountsAreEqual, TestVerifyIndexRejectsInvalidOrFailedSemanticVerification, TestVerifyIndexAcceptsExactAndOverflowingRecordUnions, TestVerifyIndexRedactsVerifierFailuresAndPreservesCancellationClass |
| `adapters/opensearch/migration_store_unix_integration_test.go` | opensearch_test | Existing evidence: TestRealOpenSearchConformancePersistsAndResumesMigratorLifecycle, TestFileMigrationStoreSerializesIndependentInstances |
| `adapters/opensearch/mixed_application_integration_test.go` | opensearch_test | Existing evidence: TestRealOpenSearchMixedApplicationProtocolVersions, TestFrozenMixedApplicationPeerBuildsOutsideAdapterModule |
| `adapters/opensearch/mutation_contract_test.go` | opensearch | Existing evidence: TestBulkWireContractAndResponseBoundaries, TestConfigurationBoundaryContract, TestConfigurationCollectionsAndConcurrencyAreAbsolutelyBounded, TestPITProjectionAndSearchResponsePredicatesAreIndependent, TestPoolRotationAndEndpointOrder, TestDiscoveryBoundaryContract, TestFailureDiagnosticAndClassificationContract, TestHealthValidationAndReadinessContract, TestCapacityValidationContract, TestInfoAndBoundedReadContract, TestLifecycleResponseBoundaryContract, TestResilienceCountersAndQueueBoundaries, TestTelemetryAndTemplateConfigurationBoundaries, TestSearchEncodingWireContract, TestSearchCursorStateAndResponseBoundaries, TestDecodeSearchResponseRejectsEachInvalidHitIdentity, TestCapabilitiesReflectLifecycleConfiguration, TestContentTypeOwnershipAndWriteStatusBoundaries |
| `adapters/opensearch/mutation_hardening_test.go` | opensearch | Existing evidence: TestSearchAcceptsCursorWithExactlyOneMillisecondRemaining, TestProjectionValidationRejectsEmptyAndNestedDisclosureEdges, TestProjectionWildcardMatcherPreservesOrderedSegmentsAndSuffixes, TestCreatePointInTimeAcceptsIdentifierAtQueryByteLimit, TestWriteResponseRejectsMismatchedNotFoundTarget, TestAppliedDeleteResultRequiresExactStatusAndResult, TestCleanupIndexValidatesEveryMigrationBinding, TestCleanupGuardFailurePreservesOutcomeKnowledgeAndOperationCause, TestPointInTimeTrackerAccountsForRebindingAcquisitionAndDefensiveRelease |
| `adapters/opensearch/operational_test.go` | opensearch_test | Existing evidence: TestOperationalArtifactsAndIncidentDrills |
| `adapters/opensearch/pit_tracker.go` | opensearch | Source API/private invariant |
| `adapters/opensearch/pit_tracker_race_test.go` | opensearch | Existing evidence: TestPointInTimeTrackerCloseMakesLaterReleaseIdempotent, TestPointInTimeTrackerRejectsDuplicateOwnershipAndConcurrentUse, TestPointInTimeTrackerNeverCallsCodecClockUnderItsLock |
| `adapters/opensearch/policy_semantics_test.go` | opensearch | Existing evidence: TestPolicyCallbacksPreserveCancellationClassification, TestSearchConfigDoesNotRequireDeprecatedClock |
| `adapters/opensearch/rebuild_checkpoint_integration_test.go` | opensearch_test | Existing evidence:  |
| `adapters/opensearch/reconcile.go` | opensearch | Source API/private invariant |
| `adapters/opensearch/reconcile_test.go` | opensearch_test | Existing evidence: TestReconciliationReaderUsesStablePITProjection, TestReconciliationReaderRejectsMissingSource |
| `adapters/opensearch/resilience.go` | opensearch | Source API/private invariant |
| `adapters/opensearch/resilience_test.go` | opensearch_test | Existing evidence: TestAdmissionRejectsExcessWorkWithoutReachingTransport, TestAdmissionRemainsHeldUntilResponseBodyIsReleased, TestCircuitBreakerStopsOverloadAmplificationUntilCooldown |
| `adapters/opensearch/search.go` | opensearch | Source API/private invariant |
| `adapters/opensearch/search_authorization_test.go` | opensearch_test | Existing evidence: TestSearchFailsClosedBeforeResolutionWithoutAuthorization, TestSearchAuthorizationReceivesExactRawQueryAndSourceScope, TestSearchAuthorizationDistinguishesFullSourceAndExplicitProjection, TestSearchAuthorizationReceivesBoundedPaginationIntent, TestSearchAuthorizationCannotMutateExecutedIntent, TestCallerCannotMutateSearchIntentAfterAuthorizationSnapshot |
| `adapters/opensearch/search_semantics_test.go` | opensearch | Existing evidence: TestEncodeBoolPreservesZeroMinimumShouldMatchSemantics, TestCursorSearchRejectsPartialPagesAndClosesOwnedPIT, TestCursorSearchClosesPITWhenAFullPageIsInvalid, TestCursorSearchRejectsExhaustedTraversalBeforeDispatch, TestCursorExpiryStartsWhenPITIsCreated, TestDuplicateCreatedPointInTimeDoesNotDeleteExistingCursor, TestCursorContinuationIsSingleConsumerPerClient, TestCursorContinuationUsesOnlyRemainingPointInTimeLifetime, TestSearchClassifiesResourceNotFoundByPaginationContext, TestCursorSearchEnforcesCumulativeBoundsAfterResponse, TestDeletePITValidatesSuccessfulResponse, TestPITMutationsClassifyAmbiguousOutcomes, TestSearchRejectsMissingRequiredResponseMetadata, TestSearchRejectsResponseOutsideRequestedShapeAndLimits, TestCursorSearchRejectsUnsafeRotatedPointInTimeIDs, TestSearchRejectsProjectionDisclosureAndPhysicalIndexMismatch, TestSearchReturnsLogicalIndexWithoutPhysicalTopology, TestCursorSearchRejectsOversizedCreatedPointInTimeBeforeSearch, TestSearchAcceptsExactResponseShapeLimits, TestSearchAcceptsHitWithoutSource, TestSearchBoundsNetworkReadByResultLimit |
| `adapters/opensearch/search_test.go` | opensearch_test | Existing evidence: TestSearchUsesPITSearchAfterAndSignedQueryBoundCursor, TestSearchBoundsAbandonedPointInTimesAndReclaimsCompletedLease, TestSearchRequestsExternalVersionsFromOpenSearch, TestSearchRejectsMoreHitsThanRequested, TestSearchRejectsInvalidAnalyzerBeforeNetworkExecution, TestCursorSearchRejectsAdapterUnsupportedQueriesBeforePITDispatch, TestSearchTranslatesTypedCapabilitiesWithoutCrossIndexLeakage, TestSearchSurfacesFinalPITCleanupFailure, TestSearchPreservesPrimaryAndPITCleanupFailures |
| `adapters/opensearch/security_integration_test.go` | opensearch_test | Existing evidence: TestRealOpenSearchSecureTLSCredentialRotationDNSAndRecovery, TestRealOpenSearchLeastPrivilegeTenantIsolation |
| `adapters/opensearch/telemetry.go` | opensearch | Source API/private invariant |
| `adapters/opensearch/telemetry_test.go` | opensearch_test | Existing evidence: TestTelemetryEmitsBoundedOperationOutcomesAndContainsObserverPanics, TestTelemetryEmitsUnknownWriteOutcomeSignal, TestTelemetryEmitsCutoverFailureSignal |
| `adapters/opensearch/template.go` | opensearch | Source API/private invariant |
| `adapters/opensearch/template_test.go` | opensearch_test | Existing evidence: TestIndexTemplatesUseAuthorizedComposableTemplateAPI, TestDeleteIndexTemplateTreatsOnlyExactMissingTemplateAsIdempotent, TestIndexTemplateOperationsRejectOversizedTenantBeforeAuthorizationOrIO |
| `adapters/opensearch/testdata/mixedappv1/main.go` | main | Frozen mixed-application v1 compatibility fixture; inspect without editing |
| `adapters/opensearch/track_location_projection_example_test.go` | opensearch_test | Existing evidence: TestTrackLocationProjectionRecipe, ExampleClient_trackLocationProjection |
| `adapters/opensearch/write.go` | opensearch | Source API/private invariant |
| `adapters/opensearch/write_authorization.go` | opensearch | Source API/private invariant |
| `adapters/opensearch/write_authorization_test.go` | opensearch_test | Existing evidence: TestWritesFailClosedWithoutDurableGuard, TestDurableWriteGuardReceivesImmutableBoundedBulkBeforeResolution, TestCallerCannotMutateWriteAfterGuardSnapshot, TestWriteGuardFailuresAreSanitizedAndClassifiedBeforeDispatch, TestBulkWriteGuardDenialStopsResolutionAndDispatch |
| `adapters/opensearch/write_test.go` | opensearch_test | Existing evidence: TestWriteUsesExternalVersioningForSupportedDocumentActions, TestWriteAndBulkRejectUpdateExistingBeforeIO, TestWriteDeleteOmitsUnsupportedRequireAliasParameter, TestWriteRejectsSuccessfulResponseWithWrongExternalVersion, TestWriteRejectsSuccessfulResponseFromAnotherPhysicalIndex, TestWriteRejectsSuccessfulResponseWithWrongResultSemantics, TestWriteDeleteRequiresExactNotFoundEvidence, TestWriteClassifiesVersionConflictOnlyFromExactBackendCode, TestWriteClassifies429ClusterBlockAsNonRetryable |
| `benchmark_test.go` | search_test | Existing evidence: BenchmarkRequestFingerprint, BenchmarkCursorRoundTrip |
| `cursor.go` | search | Source API/private invariant |
| `cursor_test.go` | search_test | Existing evidence: TestCursorRoundTripBindsQueryTenantIndexAndSnapshot, TestCursorRejectsTamperingExpiryBindingAndTraversalLimits, TestCursorCodecRejectsWeakKeysAndOversizedTokens, TestCursorDecodeRejectsUnboundedLimits, TestCursorCodecOwnsDeadlineAndRemainingLifetime |
| `document.go` | search | Source API/private invariant |
| `document_test.go` | search_test | Existing evidence: TestDocumentValidationDefinesTheOwnedBoundedWriteContract, TestDocumentRejectsHostileJSONStructure, TestDocumentAcceptsExactJSONLimitsAndPreservesUnicode, TestDocumentRejectsMissingJSONResourceLimits, TestDocumentValidationRejectsEveryUnsafeBoundary, TestDocumentRejectsInvalidUTF8IdentityAndSource |
| `example_test.go` | search_test | Existing evidence: Example |
| `fingerprint.go` | search | Source API/private invariant |
| `fingerprint_test.go` | search_test | Existing evidence: TestRequestFingerprintIsDeterministicAndBindsBehavior, TestRequestFingerprintRejectsUnboundedUnvalidatedInput |
| `fuzz_test.go` | search_test | Existing evidence: FuzzDocumentSource, FuzzIndexDefinitionJSON, FuzzCursorDecode, FuzzRequestValidateAndFingerprint |
| `internal_branches_test.go` | search | Existing evidence: TestInternalMarkerAndFingerprintModels, TestInternalQueryValidationBranches, TestInternalRequestInputBudgetBoundaries, TestInternalValueSchemaDocumentAndLimitBranches, TestInternalCursorBranches, TestInternalWriteProjectionAndResultBranches |
| `lifecycle.go` | search | Source API/private invariant |
| `lifecycle_branches_test.go` | search | Existing evidence: TestInternalLifecycleConstructionAndRunFailures, TestMigrationCoordinatorMustInvokeExactlyOnceWhileActive, TestInternalLifecycleRollbackCleanupAndLoadBranches, TestMigrationResumeRejectsDifferentPhysicalTargetWithSameDefinition, TestMigrationAcceptsExactReindexCursorBoundaries |
| `lifecycle_test.go` | search_test | Existing evidence: TestMigrationIsAuthorizedObservableResumableAndAtomicallyCutOver, TestMigrationCheckpointsEveryReindexStepAndRejectsPlanDrift, TestMigratorCoordinatesEveryWorkflowAcrossTheCompleteExternalOperation, TestMigrationRejectsUnknownPersistedPhase, TestMigrationRejectsMisattributedPersistedState, TestMigrationRejectsConflictingLifecycleNames, TestMigrationRejectsUnboundedIdentifiersAndTraversal, TestMigrationCleanupRefusesToDeleteTheActiveIndex, TestMigrationCutoverAndRollbackUseVerifiedBackendFence, TestMigrationRefusesToInferUncheckpointedCutoverOrRollback, TestMigrationDoesNotRepeatReindexAfterProgressCheckpointFailure, TestMigrationDoesNotRepeatCreateAfterCheckpointFailure, TestMigrationDoesNotRepeatCleanupAfterCheckpointFailure, TestMigrationFailsClosedWhenReindexDispatchReturnsNoResumableCursor |
| `limits.go` | search | Source API/private invariant |
| `mutation_contract_test.go` | search | Existing evidence: TestMutationContractExactScalarBoundaries, TestMutationContractJSONAndMigrationBoundaries, TestMutationContractDiagnosticBoundaries, TestMutationContractBulkPositionAttribution, TestMutationContractCursorBoundaries, TestMutationContractRequestBoundaries, TestMutationContractRequestInputBudgetExactBoundaries, TestMutationContractQueryNodeBoundaries, TestMutationContractWriteAndResultBoundaries, TestMutationContractProjectionLifecycleAndFingerprints, TestMutationContractReconciliationBoundaries |
| `projection.go` | search | Source API/private invariant |
| `projection_test.go` | search_test | Existing evidence: TestProjectionEventProducesIdempotentExternallyVersionedOperations, TestProjectionEventRejectsInvalidUTF8Identities |
| `query.go` | search | Source API/private invariant |
| `query_hardening_internal_test.go` | search | Existing evidence: TestQueryNodeInventoryAndHostileUTF8AreClosedAndRejected, TestRangeAggregationBudgetRejectsEachOversizedComponent, TestCanonicalObjectRejectsNonObjectAndTrailingValues, TestReconciliationReservationRejectsDocumentBeyondRemainingBudget |
| `query_test.go` | search_test | Existing evidence: TestRequestValidatesTypedCompositionBeforeExecution, TestRawExtensionQueryIsBoundedAndCapabilityGated, TestRequestRejectsUnsupportedAndUnstablePagination, TestRequestRejectsBackendSortEscapesAndInexactCursorDurations, TestCursorPaginationUsesBackendNeutralDocumentIDSort, TestRequestRejectsInvalidUTF8WithoutBackendNormalization, TestQueryValidationRejectsUnsafeFieldsAndRanges, TestRequestRejectsUnboundedCursorTimeQueryBytesAndOffsetOverflow, TestRequestChargesRangeBucketsToTheSharedQueryNodeBudget, TestRequestRejectsUnboundedResultFanout, TestRequestRejectsUnboundedOrBackendInvalidInputShape, TestRequestRejectsInvalidLimits |
| `reconcile.go` | search | Source API/private invariant |
| `reconcile_branches_test.go` | search | Existing evidence: TestInternalReconcilerValidationAndReaderFailures, TestReconciliationRecordBudgetUsesSharedExactRemainingBytes, TestInternalReconcilerRepairBranches, TestInternalReconcilerRejectsMisattributedRepairPosition |
| `reconcile_test.go` | search_test | Existing evidence: TestReconcilerDetectsAndRepairsMissingStaleAndOrphanedDerivedState, TestReconcilerReportsSameVersionDivergenceWithoutUnsafeOverwrite, TestSourceDigestCanonicalizesEquivalentJSONObjects, TestReconcilerBoundsCombinedRetainedRecordBytes, TestReconcilerRefusesOrphanDeletionWithoutDurableSourceGuard, TestReconcilerRejectsUnsafeDeletionGuardOutcomes, TestReconcilerDoesNotDispatchOtherRepairsWhenOrphanDeletionIsUnguarded, TestReconcilerRejectsSourceDigestThatDoesNotMatchDocument, TestReconcilerRejectsReaderPagesLargerThanRequested, TestReconcilerBoundsAndValidatesReaderTraversal, TestReconcilerRejectsInvalidUTF8ScopeBeforeReading, TestReconcilerBoundsIndexTraversalByRemainingTotal, TestReconcilerBatchesRepairsWithinTheConfiguredItemLimit, TestReconcilerRejectsMisattributedRepairResults, TestReconcilerStopsAfterAPartialRepairBatch |
| `result.go` | search | Source API/private invariant |
| `result_test.go` | search_test | Existing evidence: TestResultOwnsHitsHighlightsAggregationsAndDiagnostics, TestResultPreservesPartialFailuresAndRejectsInvalidBackendValues, TestResultRejectsAmbiguousOrMalformedBackendValues, TestResultRejectsStructurallyInvalidDiagnostics |
| `schema.go` | search | Source API/private invariant |
| `schema_test.go` | search_test | Existing evidence: TestIndexDefinitionFingerprintIsCanonicalAndCompatibilityIsExplicit, TestIndexDefinitionRejectsUnsafeNamesAndUnboundedSchema, TestIndexDefinitionRejectsHostileJSONStructureAcrossSettingsAndMappings, TestIndexDefinitionAcceptsExactJSONLimitsAndUnicode, TestIndexDefinitionRejectsMissingJSONResourceLimits, TestIndexDefinitionRejectsInvalidUTF8WithoutCanonicalReplacement |
| `searchtest/conformance.go` | searchtest | Source API/private invariant |
| `searchtest/conformance_branches_test.go` | searchtest | Existing evidence: TestConformanceRejectsInvalidConfigurationAndCapabilities, TestQueryAndTenantConformanceRejectEveryWriteAndSearchDivergence, TestQueryConformanceExercisesMatchAllAndDescendingOrdering, TestVersionAndBulkConformanceRejectMalformedAdapterOutcomes, TestRunConformanceStopsAtEveryFailedStage, TestConformanceHelpersRejectMalformedFixturesAndResults |
| `searchtest/conformance_test.go` | searchtest_test | Existing evidence: TestFakeConformsToDeclaredSharedSemantics |
| `searchtest/evidence_test.go` | searchtest_test | Existing evidence: TestEvidenceStressFakeConcurrentTenantIsolation, TestEvidenceFaultFakeStaleEventCannotResurrectDeletedSource |
| `searchtest/fake.go` | searchtest | Source API/private invariant |
| `searchtest/fake_test.go` | searchtest_test | Existing evidence: TestFakeProvidesDeterministicTenantIsolatedContractBehavior, TestFakeKeepsCompositeDocumentIdentitiesDistinct, TestFakeEnforcesExternalVersionsAndReportsPerItemOutcomes, TestFakeRetainsDeleteVersionTombstones, TestFakeMatchesOpenSearchMinimumShouldMatchDefaults, TestFakeExistsMatchesOpenSearchIndexedValueSemantics, TestFakePrefixRejectsArraysWithoutMatchingString, TestFakeRejectsDocumentsBeyondItsConfiguredCapacity |
| `searchtest/internal_branches_test.go` | searchtest | Existing evidence: TestInternalFakeConstructionCancellationAndWriteBranches, TestInternalFakeRejectsNewDeleteTombstoneAtCapacity, TestInternalFakeSearchAndMatchingBranches |
| `value.go` | search | Source API/private invariant |
| `value_test.go` | search_test | Existing evidence: TestValuesPreserveExactTypedRepresentationsAndOwnership, TestValueRejectsAmbiguousOrUnboundedInput, TestValueJSONRejectsInvalidUTF8WithoutReplacement |
| `write.go` | search | Source API/private invariant |
| `write_test.go` | search_test | Existing evidence: TestBulkRequestIsTenantIsolatedBoundedAndExternallyVersioned, TestBulkRequestRejectsUpdateWhenUpdateExistingIsUnsupported, TestBulkRequestRejectsHostileDirectWriteOperationSources, TestBulkRequestRejectsInvalidUTF8IdentityAndSource, TestBulkResultRetainsEveryPartialAndUnknownOutcome, TestBulkResultRejectsUnboundedOrInvalidDiagnostics, TestBulkResultValidatesRequestAttribution, TestBulkResultPreservesCallerBoundedDocumentIdentifiers |

## Investigation and verification disposition

Inventory preceded source edits. Every tracked source and test/support file was
read for declarations, variants, directives, helper/fixture entrypoints and
relevant assertions; tests listed above are source evidence, not new execution.
All existing canonical adapter/searchtest package comments were retained and
clarified where necessary; the root canonical comment now lives in document.go.
Public constructor/member/interface contracts and important private budget,
canonicalization, callback/ownership invariants were investigated in all 32
non-test files, with the frozen compatibility main preserved.

### Resolved missing documentation and source-backed limits

- Public Document, Request, WriteOperation, Hit, diagnostics, reconciliation and
  checkpoint structs remain mutable. Constructors/helper accessors copy selected
  retained nested data; literals/reassignments do not inherit that ownership.
  Values and results keep private copied state. Caller input must remain stable
  during validation/copy. NewResult/Value builders have no global Limits budget.
- UTF-8 validation is not normalization, authentication or authorization. Exact
  numbers retain bounded decimal spelling without exponent syntax. JSON source
  admission rejects duplicate keys at every depth; canonical definition/digest
  wire v1 uses Go JSON key ordering/number text, not RFC 8785. SourceDigest alone
  does not provide bounded admission or duplicate-key rejection. Definition
  fingerprints omit physical names and unmaterialized backend defaults.
- Query preflight bounds borrowed input work before traversal/encoding, not
  process heap, backend cost, ranking or analyzer equivalence. Closed validators
  accept value nodes/pages, not pointer forms. Result highlights/tags are untrusted
  content; text bounds do not imply redaction or safe HTML. Shared bulk bytes are
  an estimate; the adapter separately checks final NDJSON bytes.
- Search cursors use HMAC-SHA256 over plaintext JSON with strict round-trip
  RawURL-base64 spelling and fixed expiry. Key length/clock checks do not qualify
  entropy; no encryption, key-ID rotation, erasure, revocation, token consumption,
  or distributed PIT coordination is supplied. The adapter fixes expiry before
  PIT creation. Retained leases and admission are instance-local, not fleet-wide.
- Reindex cursors derive AES-256 with SHA-256 and use standard GCM/crypto-rand
  nonce prefixes. Authenticated encrypted payload binds tenant/source/target/task
  and expiry, not migration ID. Each successful incomplete poll renews its lease;
  cancellation does not cancel the server task. There is no password-hardening
  KDF, key rotation/erasure, checkpoint store or task deduplication.
- One configured RoundTripper invocation occurs per adapter-created request, not
  per public operation; custom transport internals remain caller-owned. Owned
  HTTP transports are cloned; borrowed/custom TLS and proxy behavior remains a
  trust boundary. DNS suffix rules do not pin resolved addresses. Credential,
  signer, resolver/authorization/guard/verifier and clock callbacks must be
  prompt, concurrency-safe and cooperative; typed nil, panic or blocking cannot
  be universally diagnosed/contained. Network timeouts do not encompass policy
  work that precedes request creation. Telemetry observer errors/panics are
  contained at invocation, not observer blocking or clock panics.
- Close marks closed, clears local PIT tracking and closes owned idle resources;
  it neither joins/cancels admitted operations nor deletes backend PITs. Final
  PIT deletion detaches caller cancellation but retains a fresh request timeout;
  failures are surfaced and local leases persist until later expiry reaping or
  Close. Unknown backend resources may survive until server expiry. Admission
  remains held through response-body EOF/Close, not only headers.
- Durable external-version/tombstone allocation, source outbox transactions,
  source guards, stable reader snapshots, semantic/live-definition verification,
  resource mutation exclusion, write quiescence, reader/retention/backup eligibility
  and source recovery are application obligations. Callback-count flags cannot
  prove those durable fences. Backend effects, checkpoint saves and observer
  recording are separate failure windows, not one transaction.
- Reconciliation consumes combined source/index record and retained-byte
  estimates, merges strict ID order, repairs only missing/stale and guarded
  orphan records, and never overwrites divergence. Complete can coexist with
  drift and is not cross-reader atomicity or a second zero-drift proof. Corrected
  directly contradictory current docs/consistency.md; retained history and
  selected runtime expectations unchanged.
- The fake is bounded process-local test support, retaining delete versions;
  refresh creates no delayed visibility, prefix is case-sensitive/byte-based and
  numeric term comparison uses JSON spelling. The conformance helper performs
  fixture writes and selected checks, not read-only or exhaustive qualification.
  No heap algorithm/priority queue is owned here; ordered reconciliation uses a
  two-snapshot linear merge and the fake uses sort.Slice on copied hits.

### Authorities, variants, producer and investigated question

The exact [OpenSearch policy register](../adapters/opensearch/docs/specification-decisions.md)
and [typed history rules](../adapters/opensearch/specification/README.md) retain
OPENSEARCH-DEC-001 through -023, server 2.19.6/3.8.0 and client 4.8.0.
Immutable REST/client source pins, decision/current history prefixes, official
fixtures and desired test expectations are unchanged. Source comments describe
these pins as maintained policy, not newly executed broker conformance, deployment
security review or release qualification. No external browsing/authority refresh.

Both .golib.yaml API baselines use apidiff: api/baseline.txt and the adapter's
api/baseline.txt. The native producer is the configured Tools API gate; local
scripts do not establish a full-prose snapshot. No snapshot/declaration baseline
regeneration is warranted by this comments-only change. All three package
renders cover root, searchtest and the adapter in native catalog topology.
The frozen testdata/mixedappv1/main.go is parsed/preserved, not generated or
rewritten to add private fixture documentation. Existing integration/platform
build directives, Deprecated Clock marker and lint/security directives are
preserved. No generated source or go:generate producer was found; a generator
was not fabricated/executed to satisfy a structural proxy.

Concrete investigated follow-up (maintainer-owned, not repaired here): the fake
promotes zero MinimumShouldMatch to one for should-only queries, while adapter
encodeQuery explicitly emits zero and TestEncodeBoolPreservesZeroMinimumShouldMatchSemantics
requires that zero. searchtest's conformance fixture expects only matching IDs.
These existing selected outcomes are inconsistent; runtime/backend qualification
must reconcile them separately. Current comments expose backend/default limits
rather than claiming universal shared should-default equivalence.

A second concrete coverage limitation is maintainer-owned: the existing
FuzzLifecycleResponseBoundary supplies only a lifecycle authorizer, not the
required MutationGuard. CreateIndex can therefore stop before response parsing;
its new comment exposes that limitation rather than claiming active decoder
coverage. Deterministic lifecycle malformed-response tests remain source evidence.
No runtime/helper configuration or selected expectations were repaired.

All eight fuzz boundaries and the root/synthetic/integration benchmark workload,
setup exclusions and non-equivalent comparison limits now have meaningful comments.
Their executable tokens, seeds and fixtures remain unchanged, and no campaign ran.
A third concrete source-only limitation is maintainer-owned: in adapter
withLifecycleMutation, a guard cancellation/deadline before any callback is passed
as guardErr to cancelledFailure, which stores it in Failure.cause. Error omits
that text, but Unwrap exposes the supplied chain, including any private wrapper
text. No runtime reproduction/campaign was run. Current source comments and
security/threat-model prose distinguish formatter classification from cause
exposure. Existing selected OPENSEARCH-DEC-015/-023 disclosure expectations remain
frozen for a separately scoped behavioral decision; no error-path repair is
hidden in this documentation pass.

The existing three Examples and their output bytes are unchanged and not run.
No TestMain was found; integration-tag and Unix integration fixtures nevertheless
need explicit service/filesystem ownership. All fuzz seeds, benchmarks, race,
stress/soak/leak, mixed-application source, authority/pin records and service
qualification evidence are retained without asserting new execution.

### Verification disposition

Go1.27.2 bounded native session complete: gofmt on 35 changed Go paths;
stdlib scanner/parser proof over all 105 tracked Go files preserves every
comment-free AST/signature, literal, nonoptional token and directive. Raw lexer
deltas are only five grammar-optional implicit semicolons immediately before
closing braces: PrefixQuery, ExistsQuery, OffsetPage, Projection in query.go and
ShardDiagnostics in result.go were formatted from inline structs to field-doc
blocks. No other token changed; normalization is limited to that optional grammar
position, not executable statements. Strict initial raw comparison exposed this
format-only distinction; the narrow established proof retains useful field docs
and gofmt rather than hiding it. An initial helper also assumed a nonexistent
go.work.sum; the inventory correction did not change source or dependencies.

All 819 parsed comment groups rendered to HTML, including preserved fixtures;
832 public declarations/fields/methods and three canonical package headers have
name-led/semantic structure. Native independent-module GOWORK=off go doc -all
rendered root (61133 bytes), adapter (57894 bytes before final Clock paragraph,
57898 after Clock refresh; 58212 after the final three-file clarification),
and searchtest (3975 bytes). The unchanged root and searchtest renders were
reused after the isolated adapter prose fixes. Native source
check initially required a separate Deprecated paragraph after the added Clock
contract; that was repaired without changing the historical marker line.
Final source-docs check --json reports zero findings across 105 files/two modules;
docs check --local reports local documentation valid; specification check reports
one module/23 decisions valid. The source checker is objective structure, not
semantic or runtime certification.

The native typed JSON decision-digest reproduction matched all 23 current
history tips with baseline/current records identical and all specification
files/pins/history prefixes byte-frozen. That unchanged proof was reused after
the isolated adapter prose fixes. A final bounded three-file formatting/affected
adapter render refresh repeated all-source preservation/HTML and local source/docs
gates while reusing the unchanged typed-history/offline-specification inputs.
Both structural API baselines, catalog,
module dependency files and go.work remained byte-identical; no full-prose API
snapshot mode is configured. No runnable Example, fixture/seed, dependency,
selected expectation or executable test/helper changed.

The serialized Go slot was released after exact task-owned GOCACHE, GOMODCACHE,
GOTMPDIR, helpers, probes and renders were removed on every failure/success path,
with absence asserted. Native checking used no runtime/example/crypto/fuzz/race/
benchmark/live service/consumer/release campaign. Full/changed/staged secret
scans and ordinary complete final-diff truth/usefulness review are recorded in
local delivery reporting; no external CI/publication qualification is implied.

## Objective Enforcement

The checksum-pinned public Tools v2.4.0 verifier now supplies
`make source-docs-check` as a prerequisite of the existing local and CI Make
routes. Ordinary shared CI selects strict source-documentation checking and
requires its result. Native prerequisites, operation budgets, security and
release controls are retained. Hosted exact-head evidence is recorded by the
coordination ledger; configuration alone does not establish green delivery.
