package fakeclients

import (
	"context"
	"fmt"
	"time"

	"github.com/rancher/wrangler/v3/pkg/generic"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"

	harvesterv1 "github.com/harvester/harvester/pkg/apis/harvesterhci.io/v1beta1"
	harv1type "github.com/harvester/harvester/pkg/generated/clientset/versioned/typed/harvesterhci.io/v1beta1"
	"github.com/harvester/harvester/tests/framework/fuzz"
)

type UpgradeClient func(string) harv1type.UpgradeInterface

func (c UpgradeClient) Informer() cache.SharedIndexInformer {
	//TODO implement me
	panic("implement me")
}

func (c UpgradeClient) GroupVersionKind() schema.GroupVersionKind {
	//TODO implement me
	panic("implement me")
}

func (c UpgradeClient) AddGenericHandler(_ context.Context, _ string, _ generic.Handler) {
	//TODO implement me
	panic("implement me")
}

func (c UpgradeClient) AddGenericRemoveHandler(_ context.Context, _ string, _ generic.Handler) {
	//TODO implement me
	panic("implement me")
}

func (c UpgradeClient) Updater() generic.Updater {
	//TODO implement me
	panic("implement me")
}

func (c UpgradeClient) OnChange(_ context.Context, _ string, _ generic.ObjectHandler[*harvesterv1.Upgrade]) {
	//TODO implement me
	panic("implement me")
}

func (c UpgradeClient) OnRemove(_ context.Context, _ string, _ generic.ObjectHandler[*harvesterv1.Upgrade]) {
	//TODO implement me
	panic("implement me")
}

func (c UpgradeClient) Enqueue(_, _ string) {
	//TODO implement me
	panic("implement me")
}

func (c UpgradeClient) EnqueueAfter(_, _ string, _ time.Duration) {
	// do nothing
}

func (c UpgradeClient) Cache() generic.CacheInterface[*harvesterv1.Upgrade] {
	//TODO implement me
	panic("implement me")
}

func (c UpgradeClient) Update(upgrade *harvesterv1.Upgrade) (*harvesterv1.Upgrade, error) {
	return c(upgrade.Namespace).Update(context.TODO(), upgrade, metav1.UpdateOptions{})
}
func (c UpgradeClient) Get(namespace, name string, options metav1.GetOptions) (*harvesterv1.Upgrade, error) {
	return c(namespace).Get(context.TODO(), name, options)
}
func (c UpgradeClient) Create(upgrade *harvesterv1.Upgrade) (*harvesterv1.Upgrade, error) {
	if upgrade.GenerateName != "" {
		upgrade.Name = fmt.Sprintf("%s%s", upgrade.GenerateName, fuzz.String(5))
	}
	return c(upgrade.Namespace).Create(context.TODO(), upgrade, metav1.CreateOptions{})
}
func (c UpgradeClient) Delete(_, _ string, _ *metav1.DeleteOptions) error {
	panic("implement me")
}
func (c UpgradeClient) List(_ string, _ metav1.ListOptions) (*harvesterv1.UpgradeList, error) {
	panic("implement me")
}
func (c UpgradeClient) UpdateStatus(*harvesterv1.Upgrade) (*harvesterv1.Upgrade, error) {
	panic("implement me")
}
func (c UpgradeClient) Watch(_ string, _ metav1.ListOptions) (watch.Interface, error) {
	panic("implement me")
}
func (c UpgradeClient) Patch(_, _ string, _ types.PatchType, _ []byte, _ ...string) (result *harvesterv1.Upgrade, err error) {
	panic("implement me")
}
func (c UpgradeClient) WithImpersonation(_ rest.ImpersonationConfig) (generic.ClientInterface[*harvesterv1.Upgrade, *harvesterv1.UpgradeList], error) {
	panic("implement me")
}

type UpgradeCache func(string) harv1type.UpgradeInterface

func (c UpgradeCache) Get(namespace, name string) (*harvesterv1.Upgrade, error) {
	return c(namespace).Get(context.TODO(), name, metav1.GetOptions{})
}
func (c UpgradeCache) List(namespace string, selector labels.Selector) ([]*harvesterv1.Upgrade, error) {
	list, err := c(namespace).List(context.TODO(), metav1.ListOptions{LabelSelector: selector.String()})
	if err != nil {
		return nil, err
	}
	result := make([]*harvesterv1.Upgrade, 0, len(list.Items))
	for i := range list.Items {
		result = append(result, &list.Items[i])
	}
	return result, err
}
func (c UpgradeCache) AddIndexer(_ string, _ generic.Indexer[*harvesterv1.Upgrade]) {
	// no op
}
func (c UpgradeCache) GetByIndex(key, scName string) ([]*harvesterv1.Upgrade, error) {
	panic("implement me")
}
