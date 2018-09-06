// +build k8srequired

package migration

import (
	"fmt"
	"testing"

	"github.com/giantswarm/e2e-harness/pkg/framework"
	"github.com/giantswarm/e2esetup/chart/env"
	"github.com/giantswarm/microerror"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	resourceNamespace  = metav1.NamespaceSystem
	metricsAPIEndpoint = "/apis/metrics.k8s.io/v1beta1"
)

// TestMetrics ensures that deployed metrics-server chart exposes node-metrics
// via Kubernetes API extension.
func TestMetrics(t *testing.T) {
	// Check metrics availability
	err := checkMetricsAvailability()
	if err != nil {
		t.Fatalf("could not get metrics: %v", err)
	}
}

func checkMetricsAvailability() error {
	c := h.K8sClient()

	restClient, err := c.RESTClient()
	if err != nil {
		return err
	}

	stream, err := restClient.Get().RequestURI(metricsAPIEndpoint).Stream()
	if err != nil {
		return err
	}
	defer stream.Close()

	_, err = io.Copy(o.Out, stream)
	if err != nil && err != io.EOF {
		return err
	}
	return nil
}
