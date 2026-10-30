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

type AddonClient func(string) harv1type.AddonInterface

func (c AddonClient) Informer() cache.SharedIndexInformer {
	//TODO implement me
	panic("implement me")
}

func (c AddonClient) GroupVersionKind() schema.GroupVersionKind {
	//TODO implement me
	panic("implement me")
}

func (c AddonClient) AddGenericHandler(_ context.Context, _ string, _ generic.Handler) {
	//TODO implement me
	panic("implement me")
}

func (c AddonClient) AddGenericRemoveHandler(_ context.Context, _ string, _ generic.Handler) {
	//TODO implement me
	panic("implement me")
}

func (c AddonClient) Updater() generic.Updater {
	//TODO implement me
	panic("implement me")
}

func (c AddonClient) OnChange(_ context.Context, _ string, _ generic.ObjectHandler[*harvesterv1.Addon]) {
	//TODO implement me
	panic("implement me")
}

func (c AddonClient) OnRemove(_ context.Context, _ string, _ generic.ObjectHandler[*harvesterv1.Addon]) {
	//TODO implement me
	panic("implement me")
}

func (c AddonClient) Enqueue(_, _ string) {
	//TODO implement me
	panic("implement me")
}

func (c AddonClient) EnqueueAfter(_, _ string, _ time.Duration) {
	// do nothing
}

func (c AddonClient) Cache() generic.CacheInterface[*harvesterv1.Addon] {
	//TODO implement me
	panic("implement me")
}

func (c AddonClient) Update(addon *harvesterv1.Addon) (*harvesterv1.Addon, error) {
	return c(addon.Namespace).Update(context.TODO(), addon, metav1.UpdateOptions{})
}
func (c AddonClient) Get(namespace, name string, options metav1.GetOptions) (*harvesterv1.Addon, error) {
	return c(namespace).Get(context.TODO(), name, options)
}
func (c AddonClient) Create(addon *harvesterv1.Addon) (*harvesterv1.Addon, error) {
	if addon.GenerateName != "" {
		addon.Name = fmt.Sprintf("%s%s", addon.GenerateName, fuzz.String(5))
	}
	return c(addon.Namespace).Create(context.TODO(), addon, metav1.CreateOptions{})
}
func (c AddonClient) Delete(_, _ string, _ *metav1.DeleteOptions) error {
	panic("implement me")
}
func (c AddonClient) List(_ string, _ metav1.ListOptions) (*harvesterv1.AddonList, error) {
	panic("implement me")
}
func (c AddonClient) UpdateStatus(*harvesterv1.Addon) (*harvesterv1.Addon, error) {
	panic("implement me")
}
func (c AddonClient) Watch(_ string, _ metav1.ListOptions) (watch.Interface, error) {
	panic("implement me")
}
func (c AddonClient) Patch(_, _ string, _ types.PatchType, _ []byte, _ ...string) (result *harvesterv1.Addon, err error) {
	panic("implement me")
}
func (c AddonClient) WithImpersonation(_ rest.ImpersonationConfig) (generic.ClientInterface[*harvesterv1.Addon, *harvesterv1.AddonList], error) {
	panic("implement me")
}

type AddonCache func(string) harv1type.AddonInterface

func (c AddonCache) Get(namespace, name string) (*harvesterv1.Addon, error) {
	return c(namespace).Get(context.TODO(), name, metav1.GetOptions{})
}
func (c AddonCache) List(namespace string, selector labels.Selector) ([]*harvesterv1.Addon, error) {
	list, err := c(namespace).List(context.TODO(), metav1.ListOptions{LabelSelector: selector.String()})
	if err != nil {
		return nil, err
	}
	result := make([]*harvesterv1.Addon, 0, len(list.Items))
	for i := range list.Items {
		result = append(result, &list.Items[i])
	}
	return result, err
}
func (c AddonCache) AddIndexer(_ string, _ generic.Indexer[*harvesterv1.Addon]) {
	// no op
}
func (c AddonCache) GetByIndex(key, scName string) ([]*harvesterv1.Addon, error) {
	panic("implement me")
}
