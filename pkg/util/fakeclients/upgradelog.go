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

type UpgradeLogClient func(string) harv1type.UpgradeLogInterface

func (c UpgradeLogClient) Informer() cache.SharedIndexInformer {
	//TODO implement me
	panic("implement me")
}

func (c UpgradeLogClient) GroupVersionKind() schema.GroupVersionKind {
	//TODO implement me
	panic("implement me")
}

func (c UpgradeLogClient) AddGenericHandler(_ context.Context, _ string, _ generic.Handler) {
	//TODO implement me
	panic("implement me")
}

func (c UpgradeLogClient) AddGenericRemoveHandler(_ context.Context, _ string, _ generic.Handler) {
	//TODO implement me
	panic("implement me")
}

func (c UpgradeLogClient) Updater() generic.Updater {
	//TODO implement me
	panic("implement me")
}

func (c UpgradeLogClient) OnChange(_ context.Context, _ string, _ generic.ObjectHandler[*harvesterv1.UpgradeLog]) {
	//TODO implement me
	panic("implement me")
}

func (c UpgradeLogClient) OnRemove(_ context.Context, _ string, _ generic.ObjectHandler[*harvesterv1.UpgradeLog]) {
	//TODO implement me
	panic("implement me")
}

func (c UpgradeLogClient) Enqueue(_, _ string) {
	//TODO implement me
	panic("implement me")
}

func (c UpgradeLogClient) EnqueueAfter(_, _ string, _ time.Duration) {
	// do nothing
}

func (c UpgradeLogClient) Cache() generic.CacheInterface[*harvesterv1.UpgradeLog] {
	//TODO implement me
	panic("implement me")
}

func (c UpgradeLogClient) Update(obj *harvesterv1.UpgradeLog) (*harvesterv1.UpgradeLog, error) {
	return c(obj.Namespace).Update(context.TODO(), obj, metav1.UpdateOptions{})
}
func (c UpgradeLogClient) Get(namespace, name string, options metav1.GetOptions) (*harvesterv1.UpgradeLog, error) {
	return c(namespace).Get(context.TODO(), name, options)
}
func (c UpgradeLogClient) Create(obj *harvesterv1.UpgradeLog) (*harvesterv1.UpgradeLog, error) {
	if obj.GenerateName != "" {
		obj.Name = fmt.Sprintf("%s%s", obj.GenerateName, fuzz.String(5))
	}
	return c(obj.Namespace).Create(context.TODO(), obj, metav1.CreateOptions{})
}
func (c UpgradeLogClient) Delete(_, _ string, _ *metav1.DeleteOptions) error {
	panic("implement me")
}
func (c UpgradeLogClient) List(_ string, _ metav1.ListOptions) (*harvesterv1.UpgradeLogList, error) {
	panic("implement me")
}
func (c UpgradeLogClient) UpdateStatus(*harvesterv1.UpgradeLog) (*harvesterv1.UpgradeLog, error) {
	panic("implement me")
}
func (c UpgradeLogClient) Watch(_ string, _ metav1.ListOptions) (watch.Interface, error) {
	panic("implement me")
}
func (c UpgradeLogClient) Patch(_, _ string, _ types.PatchType, _ []byte, _ ...string) (result *harvesterv1.UpgradeLog, err error) {
	panic("implement me")
}
func (c UpgradeLogClient) WithImpersonation(_ rest.ImpersonationConfig) (generic.ClientInterface[*harvesterv1.UpgradeLog, *harvesterv1.UpgradeLogList], error) {
	panic("implement me")
}

type UpgradeLogCache func(string) harv1type.UpgradeLogInterface

func (c UpgradeLogCache) Get(namespace, name string) (*harvesterv1.UpgradeLog, error) {
	return c(namespace).Get(context.TODO(), name, metav1.GetOptions{})
}
func (c UpgradeLogCache) List(namespace string, selector labels.Selector) ([]*harvesterv1.UpgradeLog, error) {
	list, err := c(namespace).List(context.TODO(), metav1.ListOptions{LabelSelector: selector.String()})
	if err != nil {
		return nil, err
	}
	result := make([]*harvesterv1.UpgradeLog, 0, len(list.Items))
	for i := range list.Items {
		result = append(result, &list.Items[i])
	}
	return result, err
}
func (c UpgradeLogCache) AddIndexer(_ string, _ generic.Indexer[*harvesterv1.UpgradeLog]) {
	// no op
}
func (c UpgradeLogCache) GetByIndex(key, scName string) ([]*harvesterv1.UpgradeLog, error) {
	panic("implement me")
}
