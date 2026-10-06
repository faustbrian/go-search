package opensearch_test

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	adapter "github.com/faustbrian/go-search/adapters/opensearch"
	"github.com/opensearch-project/opensearch-go/v4/signer/awsv2"
)

func TestOfficialAWSSignerCancellationReleasesAdmissionWithoutDispatch(t *testing.T) {
	t.Parallel()

	entered, release := make(chan struct{}), make(chan struct{})
	var releaseOnce sync.Once
	unblock := func() { releaseOnce.Do(func() { close(release) }) }
	t.Cleanup(unblock)
	var attempts, dispatched atomic.Int32
	provider := aws.CredentialsProviderFunc(func(ctx context.Context) (aws.Credentials, error) {
		if attempts.Add(1) == 1 {
			close(entered)
			select {
			case <-ctx.Done():
				return aws.Credentials{}, fmt.Errorf("private credential backend detail: %w", ctx.Err())
			case <-release:
				return aws.Credentials{}, errors.New("private credential backend detail")
			}
		}
		return aws.Credentials{AccessKeyID: "AKIDSYNTHETIC", SecretAccessKey: "synthetic-secret", SessionToken: "synthetic-token"}, nil
	})
	requestSigner, err := awsv2.NewSigner(aws.Config{Region: "eu-north-1", Credentials: provider})
	if err != nil {
		t.Fatal(err)
	}
	transport := roundTripFunc(func(*http.Request) (*http.Response, error) {
		dispatched.Add(1)
		return jsonResponse(http.StatusOK, `{"name":"node-a","cluster_name":"search","cluster_uuid":"cluster-a","version":{"number":"3.8.0"}}`), nil
	})
	client, err := adapter.New(adapter.Config{
		Endpoints: []string{"https://search.example.test"}, Signer: requestSigner,
		Transport: transport, TransportOwnership: adapter.TransportBorrowed,
		RequestTimeout: time.Minute, MaximumResponseBytes: 4096,
		Resilience: adapter.ResilienceConfig{MaximumInFlight: 1},
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = client.Close() })
	ctx, cancel := context.WithCancel(t.Context())
	t.Cleanup(cancel)
	type result struct {
		info adapter.ClusterInfo
		err  error
	}
	done := make(chan result, 1)
	go func() { info, callErr := client.Info(ctx); done <- result{info, callErr} }()
	select {
	case <-entered:
	case <-time.After(5 * time.Second):
		cancel()
		unblock()
		select {
		case <-done:
		case <-time.After(5 * time.Second):
			t.Error("request did not finish after fixture release")
		}
		t.Fatal("credential retrieval did not begin")
	}
	cancel()
	var first result
	select {
	case first = <-done:
	case <-time.After(5 * time.Second):
		unblock()
		select {
		case <-done:
		case <-time.After(5 * time.Second):
			t.Error("request did not finish after fixture release")
		}
		t.Fatal("cancelled request remained blocked in credential retrieval")
	}
	if first.info != (adapter.ClusterInfo{}) || !errors.Is(first.err, adapter.ErrTransport) || strings.Contains(first.err.Error(), "private credential backend detail") {
		t.Fatalf("cancelled Info() result/error = %#v / %v", first.info, first.err)
	}
	if dispatched.Load() != 0 {
		t.Fatalf("cancelled credential retrieval dispatched %d requests", dispatched.Load())
	}
	info, err := client.Info(t.Context())
	if err != nil || info.Node != "node-a" || info.Cluster != "search" || info.Version != "3.8.0" || dispatched.Load() != 1 {
		t.Fatalf("Info() after cancellation = %#v / %v; requests = %d", info, err, dispatched.Load())
	}
}
