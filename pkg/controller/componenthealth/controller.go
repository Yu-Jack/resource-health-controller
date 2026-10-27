package componenthealth

import (
	"context"
	"fmt"
	"reflect"
	"sort"
	"time"

	ctlharvcorev1 "github.com/harvester/harvester/pkg/generated/controllers/core/v1"
	ctlharvesterv1 "github.com/harvester/harvester/pkg/generated/controllers/harvesterhci.io/v1beta1"
	ctlkubevirtv1 "github.com/harvester/harvester/pkg/generated/controllers/kubevirt.io/v1"
	ctllonghornv1 "github.com/harvester/harvester/pkg/generated/controllers/longhorn.io/v1beta2"
	"github.com/sirupsen/logrus"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/cache"

	healthv1 "github.com/harvester/resource-health-controller/pkg/apis/health.harvesterhci.io/v1beta1"
	"github.com/harvester/resource-health-controller/pkg/config"
	ctlhealthv1 "github.com/harvester/resource-health-controller/pkg/generated/controllers/health.harvesterhci.io/v1beta1"
)

const (
	// componentName is the logical component name of the checks that watch harvester/harvester resources.
	componentName             = "harvester-controller"
	nodeComponentHealthName   = "harvester-controller-node"
	vmComponentHealthName     = "harvester-controller-vm"
	volumeComponentHealthName = "harvester-controller-volume"
)

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

// replaceComponentHealthChecks replaces all the checks of a ComponentHealth, which is owned by a single
// reconcile function, so checks that no longer apply are removed as well.
// It skips the write entirely if the resulting checks are unchanged, which is what makes the
// reconcile loop converge.
func (h *Handler) replaceComponentHealthChecks(componentHealthName string, checks map[string]healthv1.CheckResult) error {
	existing, err := h.componentHealths.Get(componentHealthName, metav1.GetOptions{})
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return fmt.Errorf("failed to get ComponentHealth %s: %w", componentHealthName, err)
		}
		created, err := h.componentHealths.Create(&healthv1.ComponentHealth{
			ObjectMeta: metav1.ObjectMeta{
				Name: componentHealthName,
				Labels: map[string]string{
					healthv1.LabelKeyComponent: componentName,
				},
			},
			Status: healthv1.ComponentHealthStatus{
				LastUpdatedAt: metav1.Now(),
				Checks:        checks,
			},
		})
		if err != nil {
			return fmt.Errorf("failed to create ComponentHealth %s: %w", componentHealthName, err)
		}
		logrus.WithFields(logrus.Fields{
			"component":        componentName,
			"component_health": componentHealthName,
			"checks":           getCheckKeys(checks),
		}).Info("created ComponentHealth checks")
		existing = created
	}

	rules, err := h.listIgnoreRules(componentHealthName)
	if err != nil {
		return fmt.Errorf("failed to list HealthIgnoreRules: %w", err)
	}
	updatedChecks, nextExpiry := applyIgnoreRules(checks, rules, time.Now())
	if nextExpiry != nil {
		// nothing else triggers a reconcile when a rule expires
		h.componentHealthController.EnqueueAfter(componentHealthName, time.Until(*nextExpiry))
	}

	if len(existing.Status.Checks) == 0 && len(updatedChecks) == 0 || reflect.DeepEqual(existing.Status.Checks, updatedChecks) {
		logrus.WithFields(logrus.Fields{
			"component":        componentName,
			"component_health": componentHealthName,
			"checks":           getCheckKeys(checks),
		}).Debug("ComponentHealth checks unchanged")
		return nil
	}

	toUpdate := existing.DeepCopy()
	if toUpdate.Labels == nil {
		toUpdate.Labels = map[string]string{}
	}
	toUpdate.Labels[healthv1.LabelKeyComponent] = componentName
	toUpdate.Status.LastUpdatedAt = metav1.Now()
	toUpdate.Status.Checks = updatedChecks
	_, err = h.componentHealths.Update(toUpdate)
	if err != nil {
		return err
	}
	logrus.WithFields(logrus.Fields{
		"component":        componentName,
		"component_health": componentHealthName,
		"checks":           getCheckKeys(checks),
	}).Info("updated ComponentHealth checks")
	return nil
}

func getCheckKeys(checks map[string]healthv1.CheckResult) []string {
	keys := make([]string, 0, len(checks))
	for key := range checks {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
