package componenthealth

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clienttesting "k8s.io/client-go/testing"
	"k8s.io/client-go/tools/cache"

	healthv1 "github.com/harvester/resource-health-controller/pkg/apis/health.harvesterhci.io/v1beta1"
	ctlhealthv1 "github.com/harvester/resource-health-controller/pkg/generated/controllers/health.harvesterhci.io/v1beta1"
	"github.com/harvester/resource-health-controller/pkg/util/fakeclients"
)

type recordingComponentHealthController struct {
	ctlhealthv1.ComponentHealthController
	enqueued      []string
	enqueuedAfter []string
}

func (controller *recordingComponentHealthController) EnqueueAfter(name string, _ time.Duration) {
	controller.enqueuedAfter = append(controller.enqueuedAfter, name)
}

func (controller *recordingComponentHealthController) Enqueue(name string) {
	controller.enqueued = append(controller.enqueued, name)
}

func TestVMBackupCallbacksEnqueueComponentHealth(t *testing.T) {
	controller := &recordingComponentHealthController{}
	handler := &vmBackupCheck{writer: componentHealthWriter{controller: controller}}
	for index := 0; index < 100; index++ {
		backup := newVMBackup(fmt.Sprintf("backup-%d", index), nil)
		result, err := handler.OnVMBackupChanged("default/"+backup.Name, backup)
		assert.Same(t, backup, result)
		assert.NoError(t, err)
	}
	assert.Len(t, controller.enqueued, 100)
	for _, name := range controller.enqueued {
		assert.Equal(t, vmBackupComponentHealthName, name)
	}
}

func TestEnqueueInitialHealth(t *testing.T) {
	controller := &recordingComponentHealthController{}
	handler := &Handler{
		componentHealthController: controller,
		healthCachesSynced:        []cache.InformerSynced{func() bool { return true }},
	}
	handler.checks = newTestHealthChecks(componentHealthWriter{})
	handler.enqueueInitialHealth(context.Background())
	assert.Equal(t, []string{
		nodeComponentHealthName,
		vmComponentHealthName,
		volumeComponentHealthName,
		vmBackupComponentHealthName,
		scheduleVMBackupComponentHealthName,
		vmImageComponentHealthName,
	}, controller.enqueued)
}

func TestEnqueueInitialHealthWaitsForCachesAndStopsOnCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	controller := &recordingComponentHealthController{}
	checked := make(chan struct{}, 1)
	handler := &Handler{
		componentHealthController: controller,
		healthCachesSynced: []cache.InformerSynced{func() bool {
			select {
			case checked <- struct{}{}:
			default:
			}
			return false
		}},
	}
	done := make(chan struct{})
	go func() {
		handler.enqueueInitialHealth(ctx)
		close(done)
	}()
	select {
	case <-checked:
	case <-time.After(5 * time.Second):
		t.Fatal("startup did not wait for cache synchronization")
	}
	cancel()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("startup did not stop on cancellation")
	}
	assert.Empty(t, controller.enqueued)
}

func TestOnComponentHealthChangedDispatch(t *testing.T) {
	tests := []struct {
		name string
	}{
		{name: nodeComponentHealthName},
		{name: vmComponentHealthName},
		{name: volumeComponentHealthName},
		{name: vmBackupComponentHealthName},
		{name: scheduleVMBackupComponentHealthName},
		{name: vmImageComponentHealthName},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			clientset, healthset := newTestClients()
			writer := componentHealthWriter{client: fakeclients.ComponentHealthClient(healthset.HealthV1beta1().ComponentHealths)}
			handler := &Handler{
				healthCachesSynced: []cache.InformerSynced{func() bool { return true }},
			}
			handler.checks = []healthCheck{
				&nodeCheck{writer: writer, nodeCache: fakeclients.NodeCache(clientset.CoreV1().Nodes)},
				&vmiCheck{writer: writer, vmiCache: fakeclients.VirtualMachineInstanceCache(clientset.KubevirtV1().VirtualMachineInstances)},
				&volumeCheck{writer: writer, volumeCache: fakeclients.LonghornVolumeCache(clientset.LonghornV1beta2().Volumes)},
				&vmBackupCheck{writer: writer, vmBackupCache: fakeclients.VMBackupCache(clientset.HarvesterhciV1beta1().VirtualMachineBackups)},
				&scheduleVMBackupCheck{writer: writer, scheduleVMBackupCache: fakeclients.SVMBackupCache(clientset.HarvesterhciV1beta1().ScheduleVMBackups)},
				&vmImageCheck{writer: writer, vmImageCache: fakeclients.VirtualMachineImageCache(clientset.HarvesterhciV1beta1().VirtualMachineImages)},
			}
			result, err := handler.OnComponentHealthChanged(tc.name, nil)
			assert.Nil(t, result)
			if !assert.NoError(t, err) {
				return
			}
			health, err := healthset.HealthV1beta1().ComponentHealths().Get(context.Background(), tc.name, metav1.GetOptions{})
			if !assert.NoError(t, err) {
				return
			}
			assert.Empty(t, health.Status.Checks)
			result, err = handler.OnComponentHealthChanged(tc.name, health)
			assert.Same(t, health, result)
			assert.NoError(t, err)
			for _, action := range healthset.Actions() {
				assert.False(t, action.Matches("update", "componenthealths"))
			}
			healths, err := healthset.HealthV1beta1().ComponentHealths().List(context.Background(), metav1.ListOptions{})
			if assert.NoError(t, err) {
				assert.Len(t, healths.Items, 1)
			}
		})
	}
}

func TestOnComponentHealthChangedWaitsForCaches(t *testing.T) {
	handler := &Handler{
		healthCachesSynced: []cache.InformerSynced{
			func() bool { return true },
			func() bool { return false },
		},
	}
	handler.checks = newTestHealthChecks(componentHealthWriter{})
	for _, check := range handler.checks {
		t.Run(check.name(), func(t *testing.T) {
			health := &healthv1.ComponentHealth{ObjectMeta: metav1.ObjectMeta{Name: check.name()}}
			result, err := handler.OnComponentHealthChanged(check.name(), health)
			assert.Same(t, health, result)
			assert.EqualError(t, err, "ComponentHealth resource caches are not synced")
		})
	}
}

func TestOnComponentHealthChangedIgnoresUnownedKeys(t *testing.T) {
	handler := &Handler{
		healthCachesSynced: []cache.InformerSynced{func() bool {
			t.Fatal("unowned keys must not check cache synchronization")
			return false
		}},
	}
	health := &healthv1.ComponentHealth{ObjectMeta: metav1.ObjectMeta{Name: "other-component"}}
	result, err := handler.OnComponentHealthChanged(health.Name, health)
	assert.Same(t, health, result)
	assert.NoError(t, err)
	result, err = handler.OnComponentHealthChanged(health.Name, nil)
	assert.Nil(t, result)
	assert.NoError(t, err)
}

func TestOnComponentHealthChangedReturnsReconcileErrors(t *testing.T) {
	clientset, _ := newTestClients()
	clientset.PrependReactor("list", "virtualmachinebackups", func(clienttesting.Action) (bool, runtime.Object, error) {
		return true, nil, fmt.Errorf("temporary cache failure")
	})
	handler := &Handler{}
	handler.checks = []healthCheck{
		&vmBackupCheck{vmBackupCache: fakeclients.VMBackupCache(clientset.HarvesterhciV1beta1().VirtualMachineBackups)},
	}
	result, err := handler.OnComponentHealthChanged(vmBackupComponentHealthName, nil)
	assert.Nil(t, result)
	assert.EqualError(t, err, "failed to list VirtualMachineBackups: temporary cache failure")
}

func TestOnComponentHealthChangedStopsUpdatingAfterChecksClear(t *testing.T) {
	health := &healthv1.ComponentHealth{
		ObjectMeta: metav1.ObjectMeta{Name: vmBackupComponentHealthName},
		Status: healthv1.ComponentHealthStatus{
			Checks: map[string]healthv1.CheckResult{
				checkKeyVMBackupFailed: {Severity: healthv1.SeverityError, Message: "stale backup failure"},
			},
		},
	}
	clientset, healthset := newTestClients(health)
	handler := &Handler{}
	writer := componentHealthWriter{client: fakeclients.ComponentHealthClient(healthset.HealthV1beta1().ComponentHealths)}
	handler.checks = []healthCheck{
		&vmBackupCheck{writer: writer, vmBackupCache: fakeclients.VMBackupCache(clientset.HarvesterhciV1beta1().VirtualMachineBackups)},
	}
	_, err := handler.OnComponentHealthChanged(health.Name, health)
	if !assert.NoError(t, err) {
		return
	}
	updated, err := healthset.HealthV1beta1().ComponentHealths().Get(context.Background(), health.Name, metav1.GetOptions{})
	if !assert.NoError(t, err) {
		return
	}
	assert.Empty(t, updated.Status.Checks)
	_, err = handler.OnComponentHealthChanged(updated.Name, updated)
	assert.NoError(t, err)
	updateCalls := 0
	for _, action := range healthset.Actions() {
		if action.Matches("update", "componenthealths") {
			updateCalls++
		}
	}
	assert.Equal(t, 1, updateCalls)
}
