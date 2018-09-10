// +build k8srequired

package metrics

import (
	"context"
	"fmt"
	"testing"
	"time"

	"k8s.io/helm/pkg/helm"

	"github.com/giantswarm/backoff"
	"github.com/giantswarm/e2esetup/chart/env"
	"github.com/giantswarm/kubernetes-metrics-server/integration/templates"
	"github.com/giantswarm/microerror"
)

const (
	longMaxInterval    = 2 * time.Minute
	metricsAPIEndpoint = "/apis/metrics.k8s.io/v1beta1"
	shortMaxInterval   = 5 * time.Second
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

	// Check metrics availability
	err = checkMetricsAvailability(ctx)
	if err != nil {
		t.Fatalf("could not get metrics: %v", err)
	}

	// Delete release
	err = helmClient.DeleteRelease(chartName, helm.DeletePurge(true))
}

func checkMetricsAvailability(ctx context.Context) error {
	var err error

	restClient := h.K8sClient().CoreV1().RESTClient()

	l.LogCtx(ctx, "level", "debug", "message", "waiting for the metrics become available")

	o := func() error {

		_, err := restClient.Get().RequestURI(metricsAPIEndpoint).Stream()
		if err != nil {
			return err
		}

		return nil
	}
	b := backoff.NewConstant(longMaxInterval, shortMaxInterval)
	n := backoff.NewNotifier(l, ctx)

	err = backoff.RetryNotify(o, b, n)
	if err != nil {
		return microerror.Mask(err)
	}

	l.LogCtx(ctx, "level", "debug", "message", "successfully retrieved metrics from metrics server")

	return nil
}
