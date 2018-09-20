// +build k8srequired

package migration

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

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	longMaxInterval    = 2 * time.Minute
	metricsAPIEndpoint = "/apis/metrics.k8s.io/v1beta1"
	resourceNamespace  = metav1.NamespaceSystem
	shortMaxInterval   = 5 * time.Second
)

// TestMigration ensures that previously deployed metrics-server chart will be deleted
// before installing managed version of the metrics-server.
func TestMigration(t *testing.T) {
	ctx := context.Background()

	// Install legacy resources.
	err := helmClient.InstallFromTarball("/e2e/fixtures/resources-chart", resourceNamespace, helm.ReleaseName("resources"))
	if err != nil {
		t.Fatalf("could not install resources chart: %v", err)
	}

	// Check legacy resources are present.
	err = checkLegacyResourcesPresent()
	if err != nil {
		t.Fatalf("legacy resources not present: %v", err)
	}

	// Check managed resources are not present.
	err = checkManagedResourcesNotPresent("app=metrics-server,giantswarm.io/service-type=managed")
	if err != nil {
		t.Fatalf("managed resources present: %v", err)
	}

	// Install kubernetes-metrics-server-chart.
	channel := fmt.Sprintf("%s-%s", env.CircleSHA(), testName)
	err = r.Install(chartName, templates.MetricsServerValues, channel)
	if err != nil {
		t.Fatalf("could not install %q %v", chartName, err)
	}

	// Wait for deployed status
	err = r.WaitForStatus(chartName, "DEPLOYED")
	if err != nil {
		t.Fatalf("timeout waiting for deployed resource: %v", err)
	}

	// Check legacy resources are not present.
	err = checkLegacyResourcesNotPresent("app!=metrics-server,giantswarm.io/service-type!=managed")
	if err != nil {
		t.Fatalf("legacy resources present: %v", err)
	}

	// Check managed resources are present.
	err = checkManagedResourcesPresent("app=metrics-server,giantswarm.io/service-type=managed")
	if err != nil {
		t.Fatalf("managed resources not present: %v", err)
	}

	// Check metrics availability
	err = checkMetricsAvailability(ctx)
	if err != nil {
		t.Fatalf("could not get metrics: %v", err)
	}

	// Delete release
	err = helmClient.DeleteRelease(chartName, helm.DeletePurge(true))
	if err != nil {
		t.Fatalf("failed to teardown resource: %v", err)
	}
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

func checkLegacyResourcesPresent() error {
	var err error

	c := h.K8sClient()
	ac := h.K8sAggregationClient()
	getOptions := metav1.GetOptions{}

	_, err = c.Rbac().ClusterRoleBindings().Get("metrics-server:system:auth-delegator", getOptions)
	if err != nil {
		return microerror.Newf("failed to get clusterrolebinding %#q: %v", "metrics-server:system:auth-delegator", err)
	}

	_, err = c.Rbac().RoleBindings(resourceNamespace).Get("metrics-server-auth-reader", getOptions)
	if err != nil {
		return microerror.Newf("failed to get rolebinding %#q/%#q: %v", resourceNamespace, "metrics-server-auth-reader", err)
	}

	_, err = ac.ApiregistrationV1beta1().APIServices().Get("v1beta1.metrics.k8s.io", getOptions)
	if err != nil {
		return microerror.Newf("failed to get apiservice %#q: %v", "v1beta1.metrics.k8s.io", err)
	}

	_, err = c.Core().ServiceAccounts(resourceNamespace).Get(metricsServerName, getOptions)
	if err != nil {
		return microerror.Newf("failed to get service account %#q: %v", "metrics-server", err)
	}

	_, err = c.Extensions().Deployments(resourceNamespace).Get(metricsServerName, getOptions)
	if err != nil {
		return microerror.Newf("failed to get deployment %#q/%#q: %v", resourceNamespace, metricsServerName, getOptions)
	}

	_, err = c.Core().Services(resourceNamespace).Get(metricsServerName, getOptions)
	if err != nil {
		return microerror.Newf("failed to get service %#q/%#q: %v", resourceNamespace, metricsServerName, getOptions)
	}

	_, err = c.Rbac().ClusterRoles().Get("system:metrics-server", getOptions)
	if err != nil {
		return microerror.Newf("failed to get clusterrole %#q: %v", "system:metrics-server", err)
	}

	_, err = c.Rbac().ClusterRoleBindings().Get("system:metrics-server", getOptions)
	if err != nil {
		return microerror.Newf("failed to get clusterrolebinding %#q: %v", "system:metrics-server", err)
	}

	return nil
}

func checkLegacyResourcesNotPresent(labelSelector string) error {
	var err error

	c := h.K8sClient()
	ac := h.K8sAggregationClient()
	listOptions := metav1.ListOptions{
		LabelSelector: fmt.Sprintf("k8s-app=metrics-server,%s", labelSelector),
	}

	crb, err := c.Rbac().ClusterRoleBindings().List(listOptions)
	if err == nil && len(crb.Items) > 0 {
		return microerror.New("expected error querying for legacy clusterrolebindings didn't happen")
	}
	if !apierrors.IsNotFound(err) {
		return microerror.Mask(err)
	}

	rb, err := c.Rbac().RoleBindings(resourceNamespace).List(listOptions)
	if err == nil && len(rb.Items) > 0 {
		return microerror.New("expected error querying for legacy rolebinding didn't happen")
	}
	if !apierrors.IsNotFound(err) {
		return microerror.Mask(err)
	}

	as, err := ac.ApiregistrationV1beta1().APIServices().List(listOptions)
	if err == nil && len(as.Items) > 0 {
		return microerror.New("expected error querying for legacy apiservice didn't happen")
	}
	if !apierrors.IsNotFound(err) {
		return microerror.Mask(err)
	}

	sa, err := c.Core().ServiceAccounts(resourceNamespace).List(listOptions)
	if err == nil && len(sa.Items) > 0 {
		return microerror.New("expected error querying for legacy serviceaccount didn't happen")
	}
	if !apierrors.IsNotFound(err) {
		return microerror.Mask(err)
	}

	d, err := c.Extensions().Deployments(resourceNamespace).List(listOptions)
	if err == nil && len(d.Items) > 0 {
		return microerror.New("expected error querying for legacy deployment didn't happen")
	}
	if !apierrors.IsNotFound(err) {
		return microerror.Mask(err)
	}

	s, err := c.Core().Services(resourceNamespace).List(listOptions)
	if err == nil && len(s.Items) > 0 {
		return microerror.New("expected error querying for legacy service didn't happen")
	}
	if !apierrors.IsNotFound(err) {
		return microerror.Mask(err)
	}

	cr, err := c.Rbac().ClusterRoles().List(listOptions)
	if err == nil && len(cr.Items) > 0 {
		return microerror.New("expected error querying for legacy clusterrole didn't happen")
	}
	if !apierrors.IsNotFound(err) {
		return microerror.Mask(err)
	}

	return nil
}

func checkManagedResourcesPresent(labelSelector string) error {
	c := h.K8sClient()
	ac := h.K8sAggregationClient()
	listOptions := metav1.ListOptions{
		LabelSelector: labelSelector,
	}

	crba, err := c.Rbac().ClusterRoleBindings().List(listOptions)
	if err != nil {
		return microerror.Mask(err)
	}
	if len(crba.Items) != 2 {
		return microerror.Newf("unexpected number of clusterrolebindings, want 2, got %d", len(crba.Items))
	}

	rba, err := c.Rbac().RoleBindings(resourceNamespace).List(listOptions)
	if err != nil {
		return microerror.Mask(err)
	}
	if len(rba.Items) != 1 {
		return microerror.Newf("unexpected number of rolebindings, want 1, got %d", len(rba.Items))
	}

	as, err := ac.ApiregistrationV1beta1().APIServices().List(listOptions)
	if err != nil {
		return microerror.Mask(err)
	}
	if len(as.Items) != 1 {
		return microerror.Newf("unexpected number of apiservices, want 1, got %d", len(as.Items))
	}

	sa, err := c.Core().ServiceAccounts(resourceNamespace).List(listOptions)
	if err != nil {
		return microerror.Mask(err)
	}
	if len(sa.Items) != 1 {
		return microerror.Newf("unexpected number of serviceaccounts, want 1, got %d", len(sa.Items))
	}

	d, err := c.Extensions().Deployments(resourceNamespace).List(listOptions)
	if err != nil {
		return microerror.Mask(err)
	}
	if len(d.Items) != 1 {
		return microerror.Newf("unexpected number of deployments, want 1, got %d", len(d.Items))
	}

	s, err := c.Core().Services(resourceNamespace).List(listOptions)
	if err != nil {
		return microerror.Mask(err)
	}
	if len(s.Items) != 1 {
		return microerror.Newf("unexpected number of services, want 1, got %d", len(s.Items))
	}

	cr, err := c.Rbac().ClusterRoles().List(listOptions)
	if err != nil {
		return microerror.Mask(err)
	}
	if len(cr.Items) != 1 {
		return microerror.Newf("unexpected number of clusterroles, want 1, got %d", len(cr.Items))
	}

	return nil
}

func checkManagedResourcesNotPresent(labelSelector string) error {
	c := h.K8sClient()
	ac := h.K8sAggregationClient()
	listOptions := metav1.ListOptions{
		LabelSelector: labelSelector,
	}

	crba, err := c.Rbac().ClusterRoleBindings().List(listOptions)
	if err == nil && len(crba.Items) > 0 {
		return microerror.New("expected error querying for managed clusterrolebindings didn't happen")
	}
	if !apierrors.IsNotFound(err) {
		return microerror.Mask(err)
	}

	rba, err := c.Rbac().RoleBindings(resourceNamespace).List(listOptions)
	if err == nil && len(rba.Items) > 0 {
		return microerror.New("expected error querying for managed rolebindings didn't happen")
	}
	if !apierrors.IsNotFound(err) {
		return microerror.Mask(err)
	}

	as, err := ac.ApiregistrationV1beta1().APIServices().List(listOptions)
	if err == nil && len(as.Items) > 0 {
		return microerror.New("expected error querying for managed apiservice didn't happen")
	}
	if !apierrors.IsNotFound(err) {
		return microerror.Mask(err)
	}

	sa, err := c.Core().ServiceAccounts(resourceNamespace).List(listOptions)
	if err == nil && len(sa.Items) > 0 {
		return microerror.New("expected error querying for managed serviceaccount didn't happen")
	}
	if !apierrors.IsNotFound(err) {
		return microerror.Mask(err)
	}

	d, err := c.Extensions().Deployments(resourceNamespace).List(listOptions)
	if err == nil && len(d.Items) > 0 {
		return microerror.New("expected error querying for managed deployment didn't happen")
	}
	if !apierrors.IsNotFound(err) {
		return microerror.Mask(err)
	}

	s, err := c.Core().Services(resourceNamespace).List(listOptions)
	if err == nil && len(s.Items) > 0 {
		return microerror.New("expected error querying for managed service didn't happen")
	}
	if !apierrors.IsNotFound(err) {
		return microerror.Mask(err)
	}

	cr, err := c.Rbac().ClusterRoles().List(listOptions)
	if err == nil && len(cr.Items) > 0 {
		return microerror.New("expected error querying for managed clusterrole didn't happen")
	}
	if !apierrors.IsNotFound(err) {
		return microerror.Mask(err)
	}

	return nil
}
