package fakeclients

import (
	"context"

	"github.com/rancher/wrangler/v3/pkg/generic"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/rest"

	healthv1 "github.com/harvester/resource-health-controller/pkg/apis/health.harvesterhci.io/v1beta1"
	healthv1type "github.com/harvester/resource-health-controller/pkg/generated/clientset/versioned/typed/health.harvesterhci.io/v1beta1"
)

type HealthSummaryClient func() healthv1type.HealthSummaryInterface

func (c HealthSummaryClient) Create(healthSummary *healthv1.HealthSummary) (*healthv1.HealthSummary, error) {
	return c().Create(context.TODO(), healthSummary, metav1.CreateOptions{})
}

func (c HealthSummaryClient) Update(healthSummary *healthv1.HealthSummary) (*healthv1.HealthSummary, error) {
	return c().Update(context.TODO(), healthSummary, metav1.UpdateOptions{})
}

func (c HealthSummaryClient) UpdateStatus(healthSummary *healthv1.HealthSummary) (*healthv1.HealthSummary, error) {
	return c().UpdateStatus(context.TODO(), healthSummary, metav1.UpdateOptions{})
}

func (c HealthSummaryClient) Delete(name string, options *metav1.DeleteOptions) error {
	return c().Delete(context.TODO(), name, *options)
}

func (c HealthSummaryClient) Get(name string, options metav1.GetOptions) (*healthv1.HealthSummary, error) {
	return c().Get(context.TODO(), name, options)
}

func (c HealthSummaryClient) List(options metav1.ListOptions) (*healthv1.HealthSummaryList, error) {
	return c().List(context.TODO(), options)
}

func (c HealthSummaryClient) Watch(options metav1.ListOptions) (watch.Interface, error) {
	return c().Watch(context.TODO(), options)
}

func (c HealthSummaryClient) Patch(name string, patchType types.PatchType, data []byte, subresources ...string) (*healthv1.HealthSummary, error) {
	return c().Patch(context.TODO(), name, patchType, data, metav1.PatchOptions{}, subresources...)
}

func (c HealthSummaryClient) WithImpersonation(_ rest.ImpersonationConfig) (generic.NonNamespacedClientInterface[*healthv1.HealthSummary, *healthv1.HealthSummaryList], error) {
	panic("implement me")
}
