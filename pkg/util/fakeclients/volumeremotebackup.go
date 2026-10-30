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

type VolumeRemoteBackupClient func(string) harv1type.VolumeRemoteBackupInterface

func (c VolumeRemoteBackupClient) Informer() cache.SharedIndexInformer {
	//TODO implement me
	panic("implement me")
}

func (c VolumeRemoteBackupClient) GroupVersionKind() schema.GroupVersionKind {
	//TODO implement me
	panic("implement me")
}

func (c VolumeRemoteBackupClient) AddGenericHandler(_ context.Context, _ string, _ generic.Handler) {
	//TODO implement me
	panic("implement me")
}

func (c VolumeRemoteBackupClient) AddGenericRemoveHandler(_ context.Context, _ string, _ generic.Handler) {
	//TODO implement me
	panic("implement me")
}

func (c VolumeRemoteBackupClient) Updater() generic.Updater {
	//TODO implement me
	panic("implement me")
}

func (c VolumeRemoteBackupClient) OnChange(_ context.Context, _ string, _ generic.ObjectHandler[*harvesterv1.VolumeRemoteBackup]) {
	//TODO implement me
	panic("implement me")
}

func (c VolumeRemoteBackupClient) OnRemove(_ context.Context, _ string, _ generic.ObjectHandler[*harvesterv1.VolumeRemoteBackup]) {
	//TODO implement me
	panic("implement me")
}

func (c VolumeRemoteBackupClient) Enqueue(_, _ string) {
	//TODO implement me
	panic("implement me")
}

func (c VolumeRemoteBackupClient) EnqueueAfter(_, _ string, _ time.Duration) {
	// do nothing
}

func (c VolumeRemoteBackupClient) Cache() generic.CacheInterface[*harvesterv1.VolumeRemoteBackup] {
	//TODO implement me
	panic("implement me")
}

func (c VolumeRemoteBackupClient) Update(obj *harvesterv1.VolumeRemoteBackup) (*harvesterv1.VolumeRemoteBackup, error) {
	return c(obj.Namespace).Update(context.TODO(), obj, metav1.UpdateOptions{})
}
func (c VolumeRemoteBackupClient) Get(namespace, name string, options metav1.GetOptions) (*harvesterv1.VolumeRemoteBackup, error) {
	return c(namespace).Get(context.TODO(), name, options)
}
func (c VolumeRemoteBackupClient) Create(obj *harvesterv1.VolumeRemoteBackup) (*harvesterv1.VolumeRemoteBackup, error) {
	if obj.GenerateName != "" {
		obj.Name = fmt.Sprintf("%s%s", obj.GenerateName, fuzz.String(5))
	}
	return c(obj.Namespace).Create(context.TODO(), obj, metav1.CreateOptions{})
}
func (c VolumeRemoteBackupClient) Delete(_, _ string, _ *metav1.DeleteOptions) error {
	panic("implement me")
}
func (c VolumeRemoteBackupClient) List(_ string, _ metav1.ListOptions) (*harvesterv1.VolumeRemoteBackupList, error) {
	panic("implement me")
}
func (c VolumeRemoteBackupClient) UpdateStatus(*harvesterv1.VolumeRemoteBackup) (*harvesterv1.VolumeRemoteBackup, error) {
	panic("implement me")
}
func (c VolumeRemoteBackupClient) Watch(_ string, _ metav1.ListOptions) (watch.Interface, error) {
	panic("implement me")
}
func (c VolumeRemoteBackupClient) Patch(_, _ string, _ types.PatchType, _ []byte, _ ...string) (result *harvesterv1.VolumeRemoteBackup, err error) {
	panic("implement me")
}
func (c VolumeRemoteBackupClient) WithImpersonation(_ rest.ImpersonationConfig) (generic.ClientInterface[*harvesterv1.VolumeRemoteBackup, *harvesterv1.VolumeRemoteBackupList], error) {
	panic("implement me")
}

type VolumeRemoteBackupCache func(string) harv1type.VolumeRemoteBackupInterface

func (c VolumeRemoteBackupCache) Get(namespace, name string) (*harvesterv1.VolumeRemoteBackup, error) {
	return c(namespace).Get(context.TODO(), name, metav1.GetOptions{})
}
func (c VolumeRemoteBackupCache) List(namespace string, selector labels.Selector) ([]*harvesterv1.VolumeRemoteBackup, error) {
	list, err := c(namespace).List(context.TODO(), metav1.ListOptions{LabelSelector: selector.String()})
	if err != nil {
		return nil, err
	}
	result := make([]*harvesterv1.VolumeRemoteBackup, 0, len(list.Items))
	for i := range list.Items {
		result = append(result, &list.Items[i])
	}
	return result, err
}
func (c VolumeRemoteBackupCache) AddIndexer(_ string, _ generic.Indexer[*harvesterv1.VolumeRemoteBackup]) {
	// no op
}
func (c VolumeRemoteBackupCache) GetByIndex(key, scName string) ([]*harvesterv1.VolumeRemoteBackup, error) {
	panic("implement me")
}
