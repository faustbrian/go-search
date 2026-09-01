# OpenSearch specification conformance

The [decision register](../docs/specification-decisions.md), [source pins](manifest.tsv), authority monitoring, machine conformance, and decision history govern the exact OpenSearch adapter matrix.

| Decision | Status | Executable boundary |
| --- | --- | --- |
| OPENSEARCH-DEC-001 | resolved | TestSupportedOpenSearchVersionsIncludesCurrentRelease |
| OPENSEARCH-DEC-002 | resolved | TestNewRejectsUnsafeOrAmbiguousTransportConfiguration |
| OPENSEARCH-DEC-003 | resolved | TestClientSelectsConfiguredNodesWithoutImplicitRetries |
| OPENSEARCH-DEC-004 | resolved | TestPoolRotationAndEndpointOrder |
| OPENSEARCH-DEC-005 | resolved | TestInfoRejectsMalformedAndOversizedResponsesWithoutLeakingBodies |
| OPENSEARCH-DEC-006 | resolved | TestSearchEncodingWireContract |
| OPENSEARCH-DEC-007 | resolved | TestSearchTranslatesTypedCapabilitiesWithoutCrossIndexLeakage |
| OPENSEARCH-DEC-008 | resolved | TestSearchEncodingWireContract |
| OPENSEARCH-DEC-009 | resolved | TestSearchUsesPITSearchAfterAndSignedQueryBoundCursor |
| OPENSEARCH-DEC-010 | resolved | TestSearchRejectsMoreHitsThanRequested |
| OPENSEARCH-DEC-011 | resolved | TestWriteUsesExternalVersioningForSupportedDocumentActions |
| OPENSEARCH-DEC-012 | resolved | TestBulkEncodesExternalVersionsAndPreservesPartialOutcomes |
| OPENSEARCH-DEC-013 | resolved | TestLifecycleImplementsCreateResumableReindexVerifyCutoverAndCleanup |
| OPENSEARCH-DEC-014 | resolved | TestIndexTemplatesUseAuthorizedComposableTemplateAPI |
| OPENSEARCH-DEC-015 | resolved | TestHealthAndCapacityPreserveOperationalSignals |
| OPENSEARCH-DEC-016 | resolved | TestFailureDiagnosticAndClassificationContract |
| OPENSEARCH-DEC-017 | resolved | TestAdmissionRejectsExcessWorkWithoutReachingTransport |
| OPENSEARCH-DEC-018 | resolved | TestRealOpenSearchConformance |
| OPENSEARCH-DEC-019 | resolved | TestSearchFailsClosedBeforeResolutionWithoutAuthorization |
| OPENSEARCH-DEC-020 | resolved | TestReindexCursorIsEncryptedBoundAndExpiring |
| OPENSEARCH-DEC-021 | resolved | TestVerifyIndexRequiresSemanticVerifierAfterCountPreflight |
| OPENSEARCH-DEC-022 | resolved | TestRealOpenSearchSnapshotRestore |
| OPENSEARCH-DEC-023 | resolved | TestAliasMutationCannotBypassActiveCleanupExclusion |

The version matrix is provider interoperability and version-differential evidence. It does not imply equivalent ranking, mappings, analyzers, plugins, managed-service extensions, or unlisted patch behavior. Release and errata feed drift blocks the online check pending review and never changes behavior automatically.
