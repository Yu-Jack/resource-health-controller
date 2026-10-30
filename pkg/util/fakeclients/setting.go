package fakeclients

import (
	"context"

	"github.com/rancher/wrangler/v3/pkg/generic"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"

	harvesterv1 "github.com/harvester/harvester/pkg/apis/harvesterhci.io/v1beta1"
	harv1type "github.com/harvester/harvester/pkg/generated/clientset/versioned/typed/harvesterhci.io/v1beta1"
)

type SettingCache func() harv1type.SettingInterface

func (c SettingCache) Get(name string) (*harvesterv1.Setting, error) {
	return c().Get(context.TODO(), name, metav1.GetOptions{})
}

func (c SettingCache) List(selector labels.Selector) ([]*harvesterv1.Setting, error) {
	list, err := c().List(context.TODO(), metav1.ListOptions{LabelSelector: selector.String()})
	if err != nil {
		return nil, err
	}
	result := make([]*harvesterv1.Setting, 0, len(list.Items))
	for _, item := range list.Items {
		obj := item
		result = append(result, &obj)
	}
	return result, err
}

func (c SettingCache) AddIndexer(_ string, _ generic.Indexer[*harvesterv1.Setting]) {
	panic("implement me")
}

func (c SettingCache) GetByIndex(_, _ string) ([]*harvesterv1.Setting, error) {
	panic("implement me")
}
