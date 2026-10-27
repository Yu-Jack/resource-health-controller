package kubevirt

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubevirtv1 "kubevirt.io/api/core/v1"

	healthv1 "github.com/harvester/resource-health-controller/pkg/apis/health.harvesterhci.io/v1beta1"
	"github.com/harvester/resource-health-controller/pkg/controller/componenthealth/healthcheck"
	"github.com/harvester/resource-health-controller/pkg/controller/componenthealth/testutil"
	"github.com/harvester/resource-health-controller/pkg/util/fakeclients"
)

func TestOnVMIChangedAfterDeletionClearsComponentHealthChecks(t *testing.T) {
	clientset, healthset := testutil.NewClients(&healthv1.ComponentHealth{
		ObjectMeta: metav1.ObjectMeta{Name: vmComponentHealthName},
		Status: healthv1.ComponentHealthStatus{
			Checks: map[string]healthv1.CheckResult{
				"StaleCheck": {Severity: healthv1.SeverityError, Message: "deleted resource failed"},
			},
		},
	})
	controller := &testutil.RecordingController{}
	writer := healthcheck.Writer{
		Controller: controller,
		Client:     fakeclients.ComponentHealthClient(healthset.HealthV1beta1().ComponentHealths),
	}
	vmis := &vmiCheck{writer: writer, vmiCache: fakeclients.VirtualMachineInstanceCache(clientset.KubevirtV1().VirtualMachineInstances)}

	result, err := vmis.OnVMIChanged("default/deleted-vmi", nil)
	assert.Nil(t, result)
	assert.NoError(t, err)
	if !assert.Equal(t, []string{vmComponentHealthName}, controller.Enqueued) {
		return
	}

	assert.NoError(t, vmis.Reconcile())

	health, err := healthset.HealthV1beta1().ComponentHealths().Get(context.Background(), vmComponentHealthName, metav1.GetOptions{})
	if assert.NoError(t, err) {
		assert.Empty(t, health.Status.Checks)
	}
}

func TestReconcileVMIOnlyUsesVMICache(t *testing.T) {
	clientset, healthset := testutil.NewClients()
	writer := healthcheck.Writer{Client: fakeclients.ComponentHealthClient(healthset.HealthV1beta1().ComponentHealths)}
	vmis := &vmiCheck{writer: writer, vmiCache: fakeclients.VirtualMachineInstanceCache(clientset.KubevirtV1().VirtualMachineInstances)}

	err := vmis.Reconcile()
	assert.NoError(t, err)

	componentHealth, err := healthset.HealthV1beta1().ComponentHealths().Get(context.Background(), vmComponentHealthName, metav1.GetOptions{})
	assert.NoError(t, err)
	assert.Equal(t, healthcheck.ComponentName, componentHealth.Labels[healthv1.LabelKeyComponent])
}

func TestGetVMINamesMatching(t *testing.T) {
	vmis := []*kubevirtv1.VirtualMachineInstance{
		{
			ObjectMeta: metav1.ObjectMeta{Namespace: "default", Name: "migratable"},
		},
		{
			ObjectMeta: metav1.ObjectMeta{Namespace: "default", Name: "host-device"},
			Spec: kubevirtv1.VirtualMachineInstanceSpec{
				Domain: kubevirtv1.DomainSpec{Devices: kubevirtv1.Devices{HostDevices: []kubevirtv1.HostDevice{{Name: "gpu"}}}},
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{Namespace: "production", Name: "vgpu"},
			Spec: kubevirtv1.VirtualMachineInstanceSpec{
				Domain: kubevirtv1.DomainSpec{Devices: kubevirtv1.Devices{GPUs: []kubevirtv1.GPU{{Name: "vgpu"}}}},
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{Namespace: "default", Name: "pinned"},
			Spec: kubevirtv1.VirtualMachineInstanceSpec{
				NodeSelector: map[string]string{corev1.LabelHostname: "harvester1"},
			},
		},
	}

	assert.Equal(t, []string{"default/host-device", "production/vgpu"}, getVMINamesMatching(vmis, hasHostDevicesOrGPUs))
	assert.Equal(t, []string{"default/pinned"}, getVMINamesMatching(vmis, hasNodeSelectorHostname))
	assert.Empty(t, getVMINamesMatching(nil, hasHostDevicesOrGPUs))
}
