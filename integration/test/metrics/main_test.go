// +build k8srequired

package metrics

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/giantswarm/apprclient"
	"github.com/giantswarm/e2e-harness/pkg/framework"
	"github.com/giantswarm/e2e-harness/pkg/framework/deployment"
	"github.com/giantswarm/e2e-harness/pkg/framework/resource"
	e2esetup "github.com/giantswarm/e2esetup/chart"
	"github.com/giantswarm/e2esetup/chart/env"
	"github.com/giantswarm/e2etests/managedservices"
	"github.com/giantswarm/helmclient"
	"github.com/giantswarm/micrologger"
	"github.com/spf13/afero"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/giantswarm/kubernetes-metrics-server/integration/templates"
)

const (
	testName = "metrics"

	metricsServerName = "metrics-server"
	chartName         = "kubernetes-metrics-server"
)

var (
	a          *apprclient.Client
	d          *deployment.Deployment
	ms         *managedservices.ManagedServices
	h          *framework.Host
	helmClient *helmclient.Client
	l          micrologger.Logger
	r          *resource.Resource
)

func init() {
	var err error

	{
		c := micrologger.Config{}
		l, err = micrologger.New(c)
		if err != nil {
			panic(err.Error())
		}
	}

	{
		c := apprclient.Config{
			Fs:     afero.NewOsFs(),
			Logger: l,

			Address:      "https://quay.io",
			Organization: "giantswarm",
		}
		a, err = apprclient.New(c)
		if err != nil {
			panic(err.Error())
		}
	}

	{
		c := framework.HostConfig{
			Logger:     l,
			ClusterID:  "na",
			VaultToken: "na",
		}
		h, err = framework.NewHost(c)
		if err != nil {
			panic(err.Error())
		}
	}

	{
		c := deployment.Config{
			K8sClient: h.K8sClient(),
			Logger:    l,
		}
		d, err = deployment.New(c)
		if err != nil {
			panic(err.Error())
		}
	}

	{
		c := helmclient.Config{
			Logger:          l,
			K8sClient:       h.K8sClient(),
			RestConfig:      h.RestConfig(),
			TillerNamespace: "giantswarm",
		}
		helmClient, err = helmclient.New(c)
		if err != nil {
			panic(err.Error())
		}
	}

	{
		c := managedservices.Config{
			ApprClient:    a,
			HelmClient:    helmClient,
			HostFramework: h,
			Logger:        l,

			ChartConfig: managedservices.ChartConfig{
				ChannelName:     fmt.Sprintf("%s-%s", env.CircleSHA(), testName),
				ChartName:       chartName,
				ChartValues:     templates.MetricsServerValues,
				Namespace:       metav1.NamespaceSystem,
				RunReleaseTests: false,
			},
			ChartResources: managedservices.ChartResources{
				Deployments: []managedservices.Deployment{
					{
						Name:      metricsServerName,
						Namespace: metav1.NamespaceSystem,
						Labels: map[string]string{
							"giantswarm.io/service-type": "managed",
							"app": metricsServerName,
						},
						MatchLabels: map[string]string{
							"app": metricsServerName,
						},
						Replicas: 1,
					},
				},
			},
		}
		ms, err = managedservices.New(c)
		if err != nil {
			panic(err.Error())
		}
	}

	resourceConfig := resource.ResourceConfig{
		Logger:     l,
		HelmClient: helmClient,
		Namespace:  metav1.NamespaceSystem,
	}
	r, err = resource.New(resourceConfig)
	if err != nil {
		panic(err.Error())
	}
}

// TestMain allows us to have common setup and teardown steps that are run
// once for all the tests https://golang.org/pkg/testing/#hdr-Main.
func TestMain(m *testing.M) {
	ctx := context.Background()

	{
		c := e2esetup.Config{
			HelmClient: helmClient,
			Host:       h,
		}

		v, err := e2esetup.Setup(ctx, m, c)
		if err != nil {
			l.LogCtx(ctx, "level", "error", "message", "e2e test failed", "stack", fmt.Sprintf("%#v\n", err))
		}
			
		os.Exit(v)
	}
}
