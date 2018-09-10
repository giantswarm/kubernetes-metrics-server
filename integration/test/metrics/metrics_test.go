// +build k8srequired

package metrics

import (
	"context"
	"fmt"
	"k8s.io/client-go/rest"
	"testing"
	"time"

	"github.com/giantswarm/e2esetup/chart/env"

	"github.com/giantswarm/kubernetes-metrics-server/integration/templates"
)

const (
	metricsAPIEndpoint = "/apis/metrics.k8s.io/v1beta1"
)

// TestMetrics ensures that deployed metrics-server chart exposes node-metrics
// via Kubernetes API extension.
func TestMetrics(t *testing.T) {
	ctx := context.Background()

	// Install resource
	err := r.InstallResource(chartName, templates.MetricsServerValues, fmt.Sprintf("%s-%s", env.CircleSHA(), testName))
	if err != nil {
		t.Fatalf("could not install resource: %v", err)
	}

	// Wait for deployed status
	err = r.WaitForStatus(chartName, "DEPLOYED")
	if err != nil {
		t.Fatalf("timeout waiting for deployed resource: %v", err)
	}

	// Wait 1 minute for metrics become available
	l.LogCtx(ctx, "level", "info", "message", "waiting 1 minute for metrics become available")
	time.Sleep(1 * time.Minute)

	// Check metrics availability
	err = checkMetricsAvailability()
	if err != nil {
		t.Fatalf("could not get metrics: %v", err)
	}
}

func checkMetricsAvailability() error {

	restConfig := h.RestConfig()
	restClient, err := rest.RESTClientFor(restConfig)
	stream, err := restClient.Get().RequestURI(metricsAPIEndpoint).Stream()
	if err != nil {
		return err
	}
	defer stream.Close()

	return nil
}
