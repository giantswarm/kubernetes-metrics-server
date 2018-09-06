// +build k8srequired

package metrics

import (
	"k8s.io/client-go/rest"
	"testing"
)

const (
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

	restConfig := h.RestConfig()
	restClient, err := rest.RESTClientFor(restConfig)
	stream, err := restClient.Get().RequestURI(metricsAPIEndpoint).Stream()
	if err != nil {
		return err
	}
	defer stream.Close()

	return nil
}
