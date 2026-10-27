package componenthealth

import (
	"context"
	"fmt"

	ctlharvcorev1 "github.com/harvester/harvester/pkg/generated/controllers/core/v1"
	ctlharvesterv1 "github.com/harvester/harvester/pkg/generated/controllers/harvesterhci.io/v1beta1"
	ctlkubevirtv1 "github.com/harvester/harvester/pkg/generated/controllers/kubevirt.io/v1"
	ctllonghornv1 "github.com/harvester/harvester/pkg/generated/controllers/longhorn.io/v1beta2"
	"k8s.io/client-go/tools/cache"

	healthv1 "github.com/harvester/resource-health-controller/pkg/apis/health.harvesterhci.io/v1beta1"
	"github.com/harvester/resource-health-controller/pkg/config"
	ctlhealthv1 "github.com/harvester/resource-health-controller/pkg/generated/controllers/health.harvesterhci.io/v1beta1"
)

const (
	nodeComponentHealthName   = "harvester-controller-node"
	vmComponentHealthName     = "harvester-controller-vm"
	volumeComponentHealthName = "harvester-controller-volume"
)

// healthCheck is a unit that owns one ComponentHealth and recomputes all of its checks.
type healthCheck interface {
	// name is the name of the ComponentHealth the check owns.
	name() string
	reconcile() error
}

// checkRegistration builds a check with its caches, registers its change handlers and returns its HasSynced.
type checkRegistration func(ctx context.Context, writer componentHealthWriter, management *config.Management) (healthCheck, cache.InformerSynced)

var componentReconcilers = []struct {
	name      string
	reconcile func(*Handler) error
}{
	{nodeComponentHealthName, (*Handler).reconcileNodes},
	{vmComponentHealthName, (*Handler).reconcileVMI},
	{volumeComponentHealthName, (*Handler).reconcileVolumes},
	{vmBackupComponentHealthName, (*Handler).reconcileVMBackups},
	{scheduleVMBackupComponentHealthName, (*Handler).reconcileScheduleVMBackups},
	{vmImageComponentHealthName, (*Handler).reconcileVMImages},
}

type Handler struct {
	componentHealthController ctlhealthv1.ComponentHealthController
	healthCachesSynced        []cache.InformerSynced
	componentHealths          ctlhealthv1.ComponentHealthClient
	componentHealthCache      ctlhealthv1.ComponentHealthCache
	nodeCache                 ctlharvcorev1.NodeCache
	vmiCache                  ctlkubevirtv1.VirtualMachineInstanceCache
	volumeCache               ctllonghornv1.VolumeCache
	vmBackupCache             ctlharvesterv1.VirtualMachineBackupCache
	scheduleVMBackupCache     ctlharvesterv1.ScheduleVMBackupCache
	vmImageCache              ctlharvesterv1.VirtualMachineImageCache
	ignoreRuleCache           ctlhealthv1.HealthIgnoreRuleCache
}

func Register(ctx context.Context, management *config.Management) error {
	componentHealths := management.HealthFactory.Health().V1beta1().ComponentHealth()
	nodes := management.CoreFactory.Core().V1().Node()
	vmis := management.KubeVirtFactory.Kubevirt().V1().VirtualMachineInstance()
	volumes := management.LonghornFactory.Longhorn().V1beta2().Volume()
	vmBackups := management.HarvesterFactory.Harvesterhci().V1beta1().VirtualMachineBackup()
	scheduleVMBackups := management.HarvesterFactory.Harvesterhci().V1beta1().ScheduleVMBackup()
	vmImages := management.HarvesterFactory.Harvesterhci().V1beta1().VirtualMachineImage()
	ignoreRules := management.HealthFactory.Health().V1beta1().HealthIgnoreRule()

	h := &Handler{
		componentHealthController: componentHealths,
		healthCachesSynced: []cache.InformerSynced{
			nodes.Informer().HasSynced,
			vmis.Informer().HasSynced,
			volumes.Informer().HasSynced,
			vmBackups.Informer().HasSynced,
			scheduleVMBackups.Informer().HasSynced,
			vmImages.Informer().HasSynced,
			ignoreRules.Informer().HasSynced,
		},
		componentHealths:      componentHealths,
		componentHealthCache:  componentHealths.Cache(),
		nodeCache:             nodes.Cache(),
		vmiCache:              vmis.Cache(),
		volumeCache:           volumes.Cache(),
		vmBackupCache:         vmBackups.Cache(),
		scheduleVMBackupCache: scheduleVMBackups.Cache(),
		vmImageCache:          vmImages.Cache(),
		ignoreRuleCache:       ignoreRules.Cache(),
	}

	// the only place checks are computed: dispatch by ComponentHealth key
	componentHealths.OnChange(ctx, "component-health-reconcile", h.OnComponentHealthChanged)

	// resource handlers only enqueue the ComponentHealth key, the workqueue de-duplicates them
	nodes.OnChange(ctx, nodeControllerName, h.OnNodeChanged)
	vmis.OnChange(ctx, vmiControllerName, h.OnVMIChanged)
	volumes.OnChange(ctx, volumeControllerName, h.OnVolumeChanged)
	vmBackups.OnChange(ctx, vmBackupControllerName, h.OnVMBackupChanged)
	scheduleVMBackups.OnChange(ctx, scheduleVMBackupControllerName, h.OnScheduleVMBackupChanged)
	vmImages.OnChange(ctx, vmImageControllerName, h.OnVMImageChanged)
	ignoreRules.OnChange(ctx, ignoreRuleControllerName, h.OnIgnoreRuleChanged)

	// seed every key once caches are synced so checks exist even when no resource changes
	go h.enqueueInitialHealth(ctx)

	return nil
}

func (h *Handler) enqueueHealth(name string) {
	h.componentHealthController.Enqueue(name)
}

func (h *Handler) enqueueInitialHealth(ctx context.Context) {
	if !cache.WaitForCacheSync(ctx.Done(), h.healthCachesSynced...) || ctx.Err() != nil {
		return
	}
	h.enqueueAllHealth()
}

func (h *Handler) enqueueAllHealth() {
	for _, component := range componentReconcilers {
		h.enqueueHealth(component.name)
	}
}

func (h *Handler) OnComponentHealthChanged(key string, health *healthv1.ComponentHealth) (*healthv1.ComponentHealth, error) {
	for _, component := range componentReconcilers {
		if component.name != key {
			continue
		}
		for _, synced := range h.healthCachesSynced {
			if !synced() {
				return health, fmt.Errorf("ComponentHealth resource caches are not synced")
			}
		}
		return health, component.reconcile(h)
	}
	return health, nil
}

// replaceComponentHealthChecks publishes checks through a componentHealthWriter built from the Handler's fields.
func (h *Handler) replaceComponentHealthChecks(componentHealthName string, checks map[string]healthv1.CheckResult) error {
	writer := componentHealthWriter{
		controller:  h.componentHealthController,
		client:      h.componentHealths,
		ignoreRules: h.ignoreRuleCache,
	}
	return writer.replaceComponentHealthChecks(componentHealthName, checks)
}
