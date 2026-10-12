// Package opensearch implements the production OpenSearch adapter for search.
// It uses opensearch-go/v4 4.8.0 for the exact OpenSearch 2.19.6/3.8.0 matrix,
// while owning request admission, authentication, decoding, and idle-resource
// lifecycle. Applications own tenant policy, durable versions, migration fences,
// semantic verification, retries, and deployment. The maintained policy register
// is docs/specification-decisions.md (OPENSEARCH-DEC-001 through -023).
//
// No background worker is started. Operator-wide Info, Discover, Health,
// Capacity, snapshots, and telemetry are not tenant-authorized APIs. Bounds are
// per client/request, not fleet-wide heap, query-cost, or shutdown deadlines.
// Callback cooperation and caller-supplied transport internals are trust boundaries.
//
// Start with New and explicit Config policies. This module introduces the official
// OpenSearch Go client and signer dependency; a configured service is required for
// real operations but not construction. No cgo or service process is started.
package opensearch

import (
	"bytes"
	"context"
	"crypto/tls"
	"errors"
	"io"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"time"
	"unicode/utf8"

	official "github.com/opensearch-project/opensearch-go/v4"
	"github.com/opensearch-project/opensearch-go/v4/signer"
)

const (
	// MaximumEndpoints bounds seed fan-out and configuration work.
	MaximumEndpoints = 32
	// MaximumDiscoveredNodes bounds one atomic topology replacement.
	MaximumDiscoveredNodes = 256
	// MaximumDecodedResponseBytes caps every buffered OpenSearch response.
	MaximumDecodedResponseBytes int64 = 64 << 20
	maximumEndpointBytes              = 2_048
	maximumCredentialBytes            = 4_096
	maximumLocaleAnalyzers            = 256
)

var (
	// ErrInvalidConfig identifies missing or internally inconsistent bounds.
	ErrInvalidConfig = errors.New("search/opensearch: configuration is invalid")
	// ErrUnsafeEndpoint identifies a seed that can disclose credentials or
	// escape the configured OpenSearch authority boundary.
	ErrUnsafeEndpoint = errors.New("search/opensearch: endpoint is unsafe")
	// ErrUnsafeProxy identifies a proxy policy that embeds credentials or does
	// not name an HTTP proxy endpoint explicitly.
	ErrUnsafeProxy = errors.New("search/opensearch: proxy is unsafe")
	// ErrInvalidTLS identifies a TLS policy that disables peer verification.
	ErrInvalidTLS = errors.New("search/opensearch: TLS configuration is invalid")
	// ErrCredentials identifies an unavailable or structurally invalid
	// credential result without exposing provider details.
	ErrCredentials = errors.New("search/opensearch: credentials are unavailable")
	// ErrContextRequired identifies an operation invoked without cancellation
	// and deadline ownership.
	ErrContextRequired = errors.New("search/opensearch: context is required")
	// ErrClosed identifies use after adapter shutdown.
	ErrClosed = errors.New("search/opensearch: client is closed")
)

// BasicCredentials is one rotation-aware authentication snapshot.
type BasicCredentials struct {
	// Username is nonempty per-request authentication material; never log it.
	Username string
	// Password is nonempty per-request authentication material; strings are not erased on
	// shutdown.
	Password string
}

// BasicCredentialsProvider supplies current least-privilege credentials per
// request. Implementations must be concurrency-safe.
type BasicCredentialsProvider interface {
	// Credentials returns one current snapshot using the request context; failures are reduced
	// to ErrCredentials.
	Credentials(context.Context) (BasicCredentials, error)
}

// TransportOwnership defines whether Close may release idle connections on a
// caller-supplied transport.
type TransportOwnership uint8

const (
	// TransportOwned transfers idle-connection cleanup to the adapter. This is
	// also the default when no transport is supplied.
	TransportOwned TransportOwnership = iota
	// TransportBorrowed keeps caller-supplied transport lifecycle ownership with
	// the caller.
	TransportBorrowed
)

// ProxyMode defines the complete outbound proxy policy.
type ProxyMode uint8

const (
	// ProxyDisabled prevents environment-driven proxying and is the default.
	ProxyDisabled ProxyMode = iota
	// ProxyEnvironment uses net/http's environment proxy policy explicitly.
	ProxyEnvironment
	// ProxyExplicit sends requests through one credential-free HTTP(S) proxy.
	ProxyExplicit
)

// ProxyPolicy defines whether and how OpenSearch requests may use a proxy.
type ProxyPolicy struct {
	// Mode selects explicit adapter-managed proxy behavior; custom transports retain their own
	// policy.
	Mode ProxyMode
	// URL is required only for ProxyExplicit and must be a credential-free HTTP(S) authority.
	URL *url.URL
}

// DiscoveryPolicy authorizes explicit topology refreshes. Discovery is
// disabled when MaximumNodes is zero. Every returned data node must match a
// DNS suffix or IP prefix before credentials can be forwarded to it.
type DiscoveryPolicy struct {
	// MaximumNodes caps accepted eligible endpoints; zero disables discovery, not seed access.
	MaximumNodes int
	// AllowedDNSSuffixes is a copied dot-prefixed lowercase hostname suffix allowlist, not
	// DNS/IP pinning.
	AllowedDNSSuffixes []string
	// AllowedCIDRs is a copied masked IP-prefix allowlist; no DNS resolution is performed for
	// hostname suffix rules.
	AllowedCIDRs []netip.Prefix
}

// Config defines explicit OpenSearch authorities and resource ownership. The
// official client's retry and environment-driven transport layers are not used;
// each adapter-created HTTP request invokes the configured RoundTripper once.
// A public operation can create multiple requests; transport internals may retry.
// Mutable configuration containers are copied, while providers, signers, clocks,
// transports, codecs, and policy callbacks are borrowed. They must be prompt,
// concurrency-safe, and context-cooperative; the adapter cannot contain all of
// their panics or forcibly stop them. Keep input graphs stable during New.
type Config struct {
	// Endpoints is a copied nonempty seed list. Normalized duplicate authorities are rejected.
	Endpoints []string

	// BasicCredentials is a borrowed per-request provider, mutually exclusive with Signer.
	BasicCredentials BasicCredentialsProvider
	// Signer is a borrowed request signer; it must honor request context and not expose
	// credentials.
	Signer signer.Signer

	// TLS is cloned for adapter-managed transports; InsecureSkipVerify here is rejected.
	// Existing custom transport trust remains caller-owned.
	TLS *tls.Config
	// Proxy configures only adapter-managed transport clones, not arbitrary borrowed internals.
	Proxy ProxyPolicy
	// Discovery is copied explicit topology-refresh authority; no periodic refresh is started.
	Discovery DiscoveryPolicy

	// Transport is borrowed or wrapped according to TransportOwnership; owned *http.Transport
	// inputs are cloned.
	Transport http.RoundTripper
	// TransportOwnership controls idle-connection cleanup, not cancellation/join of active
	// requests.
	TransportOwnership TransportOwnership

	// AllowInsecureHTTP allows unauthenticated loopback HTTP only, not arbitrary plaintext
	// authorities.
	AllowInsecureHTTP bool
	// RequestTimeout bounds each network request context; policy/resolver callbacks run before
	// that timeout.
	RequestTimeout time.Duration
	// MaximumResponseBytes caps buffered decoded response bytes, not aggregate allocation or
	// backend work.
	MaximumResponseBytes int64
	// Search is optional copied configuration; its dependencies are borrowed. Nil disables
	// search/writes.
	Search *SearchConfig
	// Lifecycle is optional copied configuration; its authorizer is required when supplied.
	Lifecycle *LifecycleConfig
	// Resilience configures instance-local admission/circuit bounds and borrowed clock.
	Resilience ResilienceConfig
	// Telemetry is optional copied observer configuration with borrowed observer/clock.
	Telemetry *TelemetryConfig
}

// Client coordinates concurrent calls when borrowed dependencies satisfy their
// concurrency contracts. Construct it with New; its zero value is unusable.
type Client struct {
	client               *official.Client
	transport            *poolTransport
	timeout              time.Duration
	maximumResponseBytes int64
	discovery            DiscoveryPolicy
	search               *SearchConfig
	lifecycle            *LifecycleConfig
	pits                 *pointInTimeTracker

	mu struct {
		sync.RWMutex
		closed bool
	}
	closeOnce sync.Once
}

// New validates configuration without network I/O and copies retained container
// state. It does not qualify credentials, callback durability/cooperation, custom
// transport TLS/proxy internals, server availability, or an actual deployment.
func New(config Config) (*Client, error) {
	addresses, insecure, err := validateConfig(config)
	if err != nil {
		return nil, err
	}
	if insecure && (config.BasicCredentials != nil || config.Signer != nil) {
		return nil, ErrUnsafeEndpoint
	}
	if !validSearchConfig(config.Search) {
		return nil, ErrInvalidConfig
	}
	if config.Lifecycle != nil && config.Lifecycle.Authorizer == nil {
		return nil, ErrInvalidConfig
	}
	telemetry, err := newTelemetry(config.Telemetry)
	if err != nil {
		return nil, err
	}
	resilience, err := newResilienceController(config.Resilience)
	if err != nil {
		return nil, err
	}

	roundTripper, err := configureTransport(config)
	if err != nil {
		return nil, err
	}
	if config.TransportOwnership == TransportBorrowed {
		roundTripper = borrowedTransport{next: roundTripper}
	} else {
		roundTripper = &ownedTransport{next: roundTripper}
	}
	endpoints := make([]*url.URL, len(addresses))
	for index, address := range addresses {
		endpoints[index], _ = url.Parse(address)
	}
	pool := &poolTransport{
		endpoints:            endpoints,
		next:                 roundTripper,
		basic:                config.BasicCredentials,
		signer:               config.Signer,
		maximumResponseBytes: config.MaximumResponseBytes,
		resilience:           resilience,
		telemetry:            telemetry,
	}
	pool.cursor.Store(^uint64(0))

	searchConfig := cloneSearchConfig(config.Search)
	client := &Client{
		client: &official.Client{Transport: pool}, transport: pool,
		timeout:              config.RequestTimeout,
		maximumResponseBytes: config.MaximumResponseBytes,
		discovery:            cloneDiscoveryPolicy(config.Discovery),
		search:               searchConfig,
		lifecycle:            cloneLifecycleConfig(config.Lifecycle),
	}
	if searchConfig != nil {
		client.pits = newPointInTimeTracker(searchConfig.CursorCodec, searchConfig.MaximumOpenPointInTimes)
	}
	return client, nil
}

func validSearchConfig(config *SearchConfig) bool {
	if config == nil {
		return true
	}
	if config.Limits.Validate() != nil || config.CursorCodec == nil || config.Resolver == nil {
		return false
	}
	if config.MaximumOpenPointInTimes < 0 || config.MaximumOpenPointInTimes > MaximumOpenPointInTimes {
		return false
	}
	if len(config.LocaleAnalyzers) > maximumLocaleAnalyzers {
		return false
	}
	for locale, analyzer := range config.LocaleAnalyzers {
		if locale == "" || len(locale) > 64 || !utf8.ValidString(locale) || strings.ContainsAny(locale, "\x00\r\n") || !analyzerPattern.MatchString(analyzer) {
			return false
		}
	}
	return true
}

func cloneSearchConfig(config *SearchConfig) *SearchConfig {
	if config == nil {
		return nil
	}
	copyConfig := *config
	copyConfig.Clock = nil
	if copyConfig.MaximumOpenPointInTimes == 0 {
		copyConfig.MaximumOpenPointInTimes = DefaultMaximumOpenPointInTimes
	}
	copyConfig.LocaleAnalyzers = make(map[string]string, len(config.LocaleAnalyzers))
	for locale, analyzer := range config.LocaleAnalyzers {
		copyConfig.LocaleAnalyzers[locale] = analyzer
	}
	return &copyConfig
}

// Close is nil-safe and idempotent, marks the client closed, clears local PIT
// tracking, and closes adapter-owned idle connections. It does not cancel or join
// admitted operations or delete backend PITs. Policy callbacks may run before a
// later closed check; already admitted transport work can finish after return.
func (c *Client) Close() error {
	if c == nil {
		return nil
	}

	var closeErr error
	c.closeOnce.Do(func() {
		c.mu.Lock()
		c.mu.closed = true
		c.mu.Unlock()
		c.pits.close()
		closeErr = c.transport.Close()
	})

	return closeErr
}

func validateConfig(config Config) ([]string, bool, error) {
	if len(config.Endpoints) == 0 || len(config.Endpoints) > MaximumEndpoints ||
		config.RequestTimeout <= 0 || config.MaximumResponseBytes <= 0 || config.MaximumResponseBytes > MaximumDecodedResponseBytes ||
		config.TransportOwnership > TransportBorrowed ||
		(config.BasicCredentials != nil && config.Signer != nil) {
		return nil, false, ErrInvalidConfig
	}
	if config.Transport == nil && config.TransportOwnership == TransportBorrowed {
		return nil, false, ErrInvalidConfig
	}
	if config.TLS != nil && config.TLS.InsecureSkipVerify {
		return nil, false, ErrInvalidTLS
	}
	if err := validateProxy(config.Proxy); err != nil {
		return nil, false, err
	}
	if err := validateDiscoveryPolicy(config.Discovery); err != nil {
		return nil, false, err
	}

	addresses := make([]string, 0, len(config.Endpoints))
	seen := make(map[string]struct{}, len(config.Endpoints))
	insecure := false
	for _, endpoint := range config.Endpoints {
		normalized, plainHTTP, err := validateEndpoint(endpoint, config.AllowInsecureHTTP)
		if err != nil {
			return nil, false, err
		}
		if _, exists := seen[normalized]; exists {
			return nil, false, ErrInvalidConfig
		}
		seen[normalized] = struct{}{}
		addresses = append(addresses, normalized)
		insecure = insecure || plainHTTP
	}

	return addresses, insecure, nil
}

func validateEndpoint(value string, allowHTTP bool) (string, bool, error) {
	if value == "" || len(value) > maximumEndpointBytes {
		return "", false, ErrUnsafeEndpoint
	}
	parsed, err := url.Parse(value)
	if err != nil || parsed.Host == "" || parsed.User != nil ||
		(parsed.Path != "" && parsed.Path != "/") || parsed.RawQuery != "" ||
		parsed.Fragment != "" {
		return "", false, ErrUnsafeEndpoint
	}

	plainHTTP := parsed.Scheme == "http"
	if parsed.Scheme != "https" && !plainHTTP {
		return "", false, ErrUnsafeEndpoint
	}
	if plainHTTP && (!allowHTTP || !loopbackHost(parsed.Hostname())) {
		return "", false, ErrUnsafeEndpoint
	}
	parsed.Path = ""
	parsed.RawPath = ""

	return strings.TrimSuffix(parsed.String(), "/"), plainHTTP, nil
}

func loopbackHost(host string) bool {
	if strings.EqualFold(host, "localhost") {
		return true
	}
	ip := net.ParseIP(host)

	return ip != nil && ip.IsLoopback()
}

func validateProxy(policy ProxyPolicy) error {
	switch policy.Mode {
	case ProxyDisabled, ProxyEnvironment:
		if policy.URL != nil {
			return ErrInvalidConfig
		}
		return nil
	case ProxyExplicit:
		if policy.URL == nil {
			return ErrInvalidConfig
		}
		if policy.URL.Host == "" || policy.URL.User != nil ||
			(policy.URL.Scheme != "http" && policy.URL.Scheme != "https") ||
			policy.URL.Path != "" || policy.URL.RawQuery != "" ||
			policy.URL.Fragment != "" {
			return ErrUnsafeProxy
		}
		return nil
	default:
		return ErrInvalidConfig
	}
}

func validateDiscoveryPolicy(policy DiscoveryPolicy) error {
	if policy.MaximumNodes < 0 {
		return ErrInvalidConfig
	}
	if policy.MaximumNodes == 0 {
		if len(policy.AllowedDNSSuffixes) != 0 || len(policy.AllowedCIDRs) != 0 {
			return ErrInvalidConfig
		}

		return nil
	}
	if policy.MaximumNodes > MaximumDiscoveredNodes {
		return ErrInvalidConfig
	}
	dnsRules, cidrRules := len(policy.AllowedDNSSuffixes), len(policy.AllowedCIDRs)
	if dnsRules > MaximumDiscoveredNodes {
		return ErrInvalidConfig
	}
	if cidrRules > MaximumDiscoveredNodes {
		return ErrInvalidConfig
	}
	if dnsRules+cidrRules > MaximumDiscoveredNodes || dnsRules+cidrRules == 0 {
		return ErrInvalidConfig
	}
	for _, suffix := range policy.AllowedDNSSuffixes {
		if len(suffix) < 2 || suffix[0] != '.' || suffix != strings.ToLower(suffix) ||
			strings.ContainsAny(suffix, "/:@\x00\r\n") {
			return ErrInvalidConfig
		}
	}
	for _, prefix := range policy.AllowedCIDRs {
		if !prefix.IsValid() || prefix != prefix.Masked() {
			return ErrInvalidConfig
		}
	}

	return nil
}

func cloneDiscoveryPolicy(policy DiscoveryPolicy) DiscoveryPolicy {
	policy.AllowedDNSSuffixes = append([]string(nil), policy.AllowedDNSSuffixes...)
	policy.AllowedCIDRs = append([]netip.Prefix(nil), policy.AllowedCIDRs...)

	return policy
}

func configureTransport(config Config) (http.RoundTripper, error) {
	if config.Transport != nil {
		if config.TransportOwnership == TransportBorrowed {
			if config.TLS != nil || config.Proxy.Mode != ProxyDisabled {
				return nil, ErrInvalidConfig
			}

			return config.Transport, nil
		}
		if template, ok := config.Transport.(*http.Transport); ok {
			return configureHTTPTransport(template.Clone(), config), nil
		}
		if config.TLS != nil || config.Proxy.Mode != ProxyDisabled {
			return nil, ErrInvalidConfig
		}

		return config.Transport, nil
	}

	return configureHTTPTransport(http.DefaultTransport.(*http.Transport).Clone(), config), nil
}

func configureHTTPTransport(transport *http.Transport, config Config) *http.Transport {
	if config.TLS != nil {
		transport.TLSClientConfig = config.TLS.Clone()
	} else if transport.TLSClientConfig != nil {
		transport.TLSClientConfig = transport.TLSClientConfig.Clone()
	} else {
		transport.TLSClientConfig = &tls.Config{}
	}
	transport.TLSClientConfig.MinVersion = max(transport.TLSClientConfig.MinVersion, uint16(tls.VersionTLS12))

	switch config.Proxy.Mode {
	case ProxyEnvironment:
		transport.Proxy = http.ProxyFromEnvironment
	case ProxyExplicit:
		proxy := *config.Proxy.URL
		transport.Proxy = http.ProxyURL(&proxy)
	default:
		transport.Proxy = nil
	}

	return transport
}

type poolTransport struct {
	endpoints            []*url.URL
	next                 http.RoundTripper
	basic                BasicCredentialsProvider
	signer               signer.Signer
	maximumResponseBytes int64
	resilience           *resilienceController
	telemetry            *telemetry
	cursor               atomic.Uint64
	mu                   sync.RWMutex
}

func (transport *poolTransport) Perform(request *http.Request) (*http.Response, error) {
	response, err := transport.Stream(request)
	if response == nil || response.Body == nil {
		return response, err
	}
	body, readErr := readBounded(response.Body, transport.maximumResponseBytes)
	closeErr := response.Body.Close()
	response.Body = io.NopCloser(bytes.NewReader(body))
	if err != nil {
		return response, err
	}
	if readErr != nil {
		return response, readErr
	}
	if closeErr != nil {
		return response, ErrMalformedResponse
	}

	return response, nil
}

func (transport *poolTransport) Stream(request *http.Request) (*http.Response, error) {
	operation := operationFromContext(request.Context())
	started := transport.telemetry.now()
	permit, err := transport.resilience.acquire(request.Context())
	if err != nil {
		transport.telemetry.observe(request.Context(), transport.telemetry.event(operation, started, 0, err, transport.resilience.snapshot()))
		return nil, err
	}
	completed := false
	defer func() {
		if !completed {
			permit.complete(nil, nil, false)
		}
	}()
	request = request.Clone(request.Context())
	request.Header = request.Header.Clone()
	transport.mu.RLock()
	endpoint := transport.endpoints[transport.cursor.Add(1)%uint64(len(transport.endpoints))]
	resolved := *endpoint
	transport.mu.RUnlock()
	resolved.Path = request.URL.Path
	resolved.RawPath = request.URL.RawPath
	resolved.RawQuery = request.URL.RawQuery
	request.URL = &resolved
	request.Host = ""

	if transport.basic != nil {
		credentials, err := transport.basic.Credentials(request.Context())
		if err != nil || !validCredentials(credentials) {
			transport.telemetry.observe(request.Context(), transport.telemetry.event(operation, started, 0, ErrCredentials, transport.resilience.snapshot()))
			return nil, ErrCredentials
		}
		request.SetBasicAuth(credentials.Username, credentials.Password)
	}
	if transport.signer != nil {
		if err := transport.signer.SignRequest(request); err != nil {
			transport.telemetry.observe(request.Context(), transport.telemetry.event(operation, started, 0, ErrCredentials, transport.resilience.snapshot()))
			return nil, ErrCredentials
		}
	}

	response, roundTripErr := transport.next.RoundTrip(request)
	status := 0
	if response != nil {
		status = response.StatusCode
	}
	finish := func() {
		permit.complete(response, roundTripErr, true)
		transport.telemetry.observe(request.Context(), transport.telemetry.event(operation, started, status, roundTripErr, transport.resilience.snapshot()))
	}
	if response == nil || response.Body == nil {
		finish()
	} else {
		response.Body = &admissionResponseBody{ReadCloser: response.Body, finish: finish}
	}
	completed = true
	return response, roundTripErr
}

// Admission covers body lifetime, not just response headers. EOF or Close
// releases the permit once; a body that never cooperates can retain capacity.
type admissionResponseBody struct {
	io.ReadCloser
	finishOnce sync.Once
	finish     func()
}

func (body *admissionResponseBody) Read(buffer []byte) (int, error) {
	read, err := body.ReadCloser.Read(buffer)
	if errors.Is(err, io.EOF) {
		body.finishOnce.Do(body.finish)
	}
	return read, err
}

func (body *admissionResponseBody) Close() error {
	err := body.ReadCloser.Close()
	body.finishOnce.Do(body.finish)
	return err
}

func (transport *poolTransport) replaceEndpoints(endpoints []*url.URL) {
	transport.mu.Lock()
	transport.endpoints = endpoints
	transport.cursor.Store(^uint64(0))
	transport.mu.Unlock()
}

func (transport *poolTransport) endpointScheme() string {
	transport.mu.RLock()
	defer transport.mu.RUnlock()

	return transport.endpoints[0].Scheme
}

func (transport *poolTransport) Close() error {
	closeIdle(transport.next)

	return nil
}

func validCredentials(credentials BasicCredentials) bool {
	return credentials.Username != "" && credentials.Password != "" &&
		len(credentials.Username) <= maximumCredentialBytes &&
		len(credentials.Password) <= maximumCredentialBytes &&
		!strings.ContainsAny(credentials.Username, ":\r\n") &&
		!strings.ContainsAny(credentials.Password, "\r\n")
}

type borrowedTransport struct{ next http.RoundTripper }

func (transport borrowedTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	return transport.next.RoundTrip(request)
}

type ownedTransport struct {
	next http.RoundTripper
	once sync.Once
}

func (transport *ownedTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	return transport.next.RoundTrip(request)
}

func (transport *ownedTransport) CloseIdleConnections() {
	transport.once.Do(func() { closeIdle(transport.next) })
}

func closeIdle(transport http.RoundTripper) {
	if closer, ok := transport.(interface{ CloseIdleConnections() }); ok {
		closer.CloseIdleConnections()
	}
}
