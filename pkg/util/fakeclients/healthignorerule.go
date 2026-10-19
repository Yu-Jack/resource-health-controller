package fakeclients

import (
	"context"

	"github.com/rancher/wrangler/v3/pkg/generic"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"

	healthv1 "github.com/harvester/resource-health-controller/pkg/apis/health.harvesterhci.io/v1beta1"
	healthv1type "github.com/harvester/resource-health-controller/pkg/generated/clientset/versioned/typed/health.harvesterhci.io/v1beta1"
)

type HealthIgnoreRuleCache func() healthv1type.HealthIgnoreRuleInterface

func (c HealthIgnoreRuleCache) Get(name string) (*healthv1.HealthIgnoreRule, error) {
	return c().Get(context.TODO(), name, metav1.GetOptions{})
}

func (c HealthIgnoreRuleCache) List(selector labels.Selector) ([]*healthv1.HealthIgnoreRule, error) {
	list, err := c().List(context.TODO(), metav1.ListOptions{LabelSelector: selector.String()})
	if err != nil {
		return nil, err
	}
	result := make([]*healthv1.HealthIgnoreRule, 0, len(list.Items))
	for index := range list.Items {
		result = append(result, &list.Items[index])
	}
	return result, nil
}

func (c HealthIgnoreRuleCache) AddIndexer(_ string, _ generic.Indexer[*healthv1.HealthIgnoreRule]) {
	panic("implement me")
}

func (c HealthIgnoreRuleCache) GetByIndex(_, _ string) ([]*healthv1.HealthIgnoreRule, error) {
	panic("implement me")
}
