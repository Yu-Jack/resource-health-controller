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

type VolumeRemoteRestoreClient func(string) harv1type.VolumeRemoteRestoreInterface

func (c VolumeRemoteRestoreClient) Informer() cache.SharedIndexInformer {
	//TODO implement me
	panic("implement me")
}

func (c VolumeRemoteRestoreClient) GroupVersionKind() schema.GroupVersionKind {
	//TODO implement me
	panic("implement me")
}

func (c VolumeRemoteRestoreClient) AddGenericHandler(_ context.Context, _ string, _ generic.Handler) {
	//TODO implement me
	panic("implement me")
}

func (c VolumeRemoteRestoreClient) AddGenericRemoveHandler(_ context.Context, _ string, _ generic.Handler) {
	//TODO implement me
	panic("implement me")
}

func (c VolumeRemoteRestoreClient) Updater() generic.Updater {
	//TODO implement me
	panic("implement me")
}

func (c VolumeRemoteRestoreClient) OnChange(_ context.Context, _ string, _ generic.ObjectHandler[*harvesterv1.VolumeRemoteRestore]) {
	//TODO implement me
	panic("implement me")
}

func (c VolumeRemoteRestoreClient) OnRemove(_ context.Context, _ string, _ generic.ObjectHandler[*harvesterv1.VolumeRemoteRestore]) {
	//TODO implement me
	panic("implement me")
}

func (c VolumeRemoteRestoreClient) Enqueue(_, _ string) {
	//TODO implement me
	panic("implement me")
}

func (c VolumeRemoteRestoreClient) EnqueueAfter(_, _ string, _ time.Duration) {
	// do nothing
}

func (c VolumeRemoteRestoreClient) Cache() generic.CacheInterface[*harvesterv1.VolumeRemoteRestore] {
	//TODO implement me
	panic("implement me")
}

func (c VolumeRemoteRestoreClient) Update(obj *harvesterv1.VolumeRemoteRestore) (*harvesterv1.VolumeRemoteRestore, error) {
	return c(obj.Namespace).Update(context.TODO(), obj, metav1.UpdateOptions{})
}
func (c VolumeRemoteRestoreClient) Get(namespace, name string, options metav1.GetOptions) (*harvesterv1.VolumeRemoteRestore, error) {
	return c(namespace).Get(context.TODO(), name, options)
}
func (c VolumeRemoteRestoreClient) Create(obj *harvesterv1.VolumeRemoteRestore) (*harvesterv1.VolumeRemoteRestore, error) {
	if obj.GenerateName != "" {
		obj.Name = fmt.Sprintf("%s%s", obj.GenerateName, fuzz.String(5))
	}
	return c(obj.Namespace).Create(context.TODO(), obj, metav1.CreateOptions{})
}
func (c VolumeRemoteRestoreClient) Delete(_, _ string, _ *metav1.DeleteOptions) error {
	panic("implement me")
}
func (c VolumeRemoteRestoreClient) List(_ string, _ metav1.ListOptions) (*harvesterv1.VolumeRemoteRestoreList, error) {
	panic("implement me")
}
func (c VolumeRemoteRestoreClient) UpdateStatus(*harvesterv1.VolumeRemoteRestore) (*harvesterv1.VolumeRemoteRestore, error) {
	panic("implement me")
}
func (c VolumeRemoteRestoreClient) Watch(_ string, _ metav1.ListOptions) (watch.Interface, error) {
	panic("implement me")
}
func (c VolumeRemoteRestoreClient) Patch(_, _ string, _ types.PatchType, _ []byte, _ ...string) (result *harvesterv1.VolumeRemoteRestore, err error) {
	panic("implement me")
}
func (c VolumeRemoteRestoreClient) WithImpersonation(_ rest.ImpersonationConfig) (generic.ClientInterface[*harvesterv1.VolumeRemoteRestore, *harvesterv1.VolumeRemoteRestoreList], error) {
	panic("implement me")
}

type VolumeRemoteRestoreCache func(string) harv1type.VolumeRemoteRestoreInterface

func (c VolumeRemoteRestoreCache) Get(namespace, name string) (*harvesterv1.VolumeRemoteRestore, error) {
	return c(namespace).Get(context.TODO(), name, metav1.GetOptions{})
}
func (c VolumeRemoteRestoreCache) List(namespace string, selector labels.Selector) ([]*harvesterv1.VolumeRemoteRestore, error) {
	list, err := c(namespace).List(context.TODO(), metav1.ListOptions{LabelSelector: selector.String()})
	if err != nil {
		return nil, err
	}
	result := make([]*harvesterv1.VolumeRemoteRestore, 0, len(list.Items))
	for i := range list.Items {
		result = append(result, &list.Items[i])
	}
	return result, err
}
func (c VolumeRemoteRestoreCache) AddIndexer(_ string, _ generic.Indexer[*harvesterv1.VolumeRemoteRestore]) {
	// no op
}
func (c VolumeRemoteRestoreCache) GetByIndex(key, scName string) ([]*harvesterv1.VolumeRemoteRestore, error) {
	panic("implement me")
}
