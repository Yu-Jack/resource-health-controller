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

type SupportBundleClient func(string) harv1type.SupportBundleInterface

func (c SupportBundleClient) Informer() cache.SharedIndexInformer {
	//TODO implement me
	panic("implement me")
}

func (c SupportBundleClient) GroupVersionKind() schema.GroupVersionKind {
	//TODO implement me
	panic("implement me")
}

func (c SupportBundleClient) AddGenericHandler(_ context.Context, _ string, _ generic.Handler) {
	//TODO implement me
	panic("implement me")
}

func (c SupportBundleClient) AddGenericRemoveHandler(_ context.Context, _ string, _ generic.Handler) {
	//TODO implement me
	panic("implement me")
}

func (c SupportBundleClient) Updater() generic.Updater {
	//TODO implement me
	panic("implement me")
}

func (c SupportBundleClient) OnChange(_ context.Context, _ string, _ generic.ObjectHandler[*harvesterv1.SupportBundle]) {
	//TODO implement me
	panic("implement me")
}

func (c SupportBundleClient) OnRemove(_ context.Context, _ string, _ generic.ObjectHandler[*harvesterv1.SupportBundle]) {
	//TODO implement me
	panic("implement me")
}

func (c SupportBundleClient) Enqueue(_, _ string) {
	//TODO implement me
	panic("implement me")
}

func (c SupportBundleClient) EnqueueAfter(_, _ string, _ time.Duration) {
	// do nothing
}

func (c SupportBundleClient) Cache() generic.CacheInterface[*harvesterv1.SupportBundle] {
	//TODO implement me
	panic("implement me")
}

func (c SupportBundleClient) Update(obj *harvesterv1.SupportBundle) (*harvesterv1.SupportBundle, error) {
	return c(obj.Namespace).Update(context.TODO(), obj, metav1.UpdateOptions{})
}
func (c SupportBundleClient) Get(namespace, name string, options metav1.GetOptions) (*harvesterv1.SupportBundle, error) {
	return c(namespace).Get(context.TODO(), name, options)
}
func (c SupportBundleClient) Create(obj *harvesterv1.SupportBundle) (*harvesterv1.SupportBundle, error) {
	if obj.GenerateName != "" {
		obj.Name = fmt.Sprintf("%s%s", obj.GenerateName, fuzz.String(5))
	}
	return c(obj.Namespace).Create(context.TODO(), obj, metav1.CreateOptions{})
}
func (c SupportBundleClient) Delete(_, _ string, _ *metav1.DeleteOptions) error {
	panic("implement me")
}
func (c SupportBundleClient) List(_ string, _ metav1.ListOptions) (*harvesterv1.SupportBundleList, error) {
	panic("implement me")
}
func (c SupportBundleClient) UpdateStatus(*harvesterv1.SupportBundle) (*harvesterv1.SupportBundle, error) {
	panic("implement me")
}
func (c SupportBundleClient) Watch(_ string, _ metav1.ListOptions) (watch.Interface, error) {
	panic("implement me")
}
func (c SupportBundleClient) Patch(_, _ string, _ types.PatchType, _ []byte, _ ...string) (result *harvesterv1.SupportBundle, err error) {
	panic("implement me")
}
func (c SupportBundleClient) WithImpersonation(_ rest.ImpersonationConfig) (generic.ClientInterface[*harvesterv1.SupportBundle, *harvesterv1.SupportBundleList], error) {
	panic("implement me")
}

type SupportBundleCache func(string) harv1type.SupportBundleInterface

func (c SupportBundleCache) Get(namespace, name string) (*harvesterv1.SupportBundle, error) {
	return c(namespace).Get(context.TODO(), name, metav1.GetOptions{})
}
func (c SupportBundleCache) List(namespace string, selector labels.Selector) ([]*harvesterv1.SupportBundle, error) {
	list, err := c(namespace).List(context.TODO(), metav1.ListOptions{LabelSelector: selector.String()})
	if err != nil {
		return nil, err
	}
	result := make([]*harvesterv1.SupportBundle, 0, len(list.Items))
	for i := range list.Items {
		result = append(result, &list.Items[i])
	}
	return result, err
}
func (c SupportBundleCache) AddIndexer(_ string, _ generic.Indexer[*harvesterv1.SupportBundle]) {
	// no op
}
func (c SupportBundleCache) GetByIndex(key, scName string) ([]*harvesterv1.SupportBundle, error) {
	panic("implement me")
}
