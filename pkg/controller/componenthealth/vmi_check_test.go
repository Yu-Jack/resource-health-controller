package componenthealth

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubevirtv1 "kubevirt.io/api/core/v1"

	healthv1 "github.com/harvester/resource-health-controller/pkg/apis/health.harvesterhci.io/v1beta1"
	"github.com/harvester/resource-health-controller/pkg/util/fakeclients"
)

func TestReconcileVMIOnlyUsesVMICache(t *testing.T) {
	clientset, healthset := newTestClients()
	handler := &Handler{
		componentHealths: fakeclients.ComponentHealthClient(healthset.HealthV1beta1().ComponentHealths),
		vmiCache:         fakeclients.VirtualMachineInstanceCache(clientset.KubevirtV1().VirtualMachineInstances),
	}

	err := handler.reconcileVMI()
	assert.NoError(t, err)

	componentHealth, err := healthset.HealthV1beta1().ComponentHealths().Get(context.Background(), vmComponentHealthName, metav1.GetOptions{})
	assert.NoError(t, err)
	assert.Equal(t, componentName, componentHealth.Labels[healthv1.LabelKeyComponent])
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
