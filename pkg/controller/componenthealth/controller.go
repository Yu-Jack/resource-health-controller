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
	// minSeverity hides the checks below this severity, empty shows all of them.
	minSeverity healthv1.Severity
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
		minSeverity:           management.MinSeverity,
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
	for _, name := range []string{
		nodeComponentHealthName,
		vmComponentHealthName,
		volumeComponentHealthName,
		vmBackupComponentHealthName,
		scheduleVMBackupComponentHealthName,
		vmImageComponentHealthName,
	} {
		h.enqueueHealth(name)
	}
}

func (h *Handler) OnComponentHealthChanged(key string, health *healthv1.ComponentHealth) (*healthv1.ComponentHealth, error) {
	var reconcile func() error
	switch key {
	case nodeComponentHealthName:
		reconcile = h.reconcileNodes
	case vmComponentHealthName:
		reconcile = h.reconcileVMI
	case volumeComponentHealthName:
		reconcile = h.reconcileVolumes
	case vmBackupComponentHealthName:
		reconcile = h.reconcileVMBackups
	case scheduleVMBackupComponentHealthName:
		reconcile = h.reconcileScheduleVMBackups
	case vmImageComponentHealthName:
		reconcile = h.reconcileVMImages
	default:
		return health, nil
	}
	for _, synced := range h.healthCachesSynced {
		if !synced() {
			return health, fmt.Errorf("ComponentHealth resource caches are not synced")
		}
	}
	return health, reconcile()
}

// replaceComponentHealthChecks replaces all the checks of a ComponentHealth that is owned by a single
// reconcile function, so checks that no longer apply are removed as well.
func (h *Handler) replaceComponentHealthChecks(componentHealthName string, checks map[string]healthv1.CheckResult) error {
	return h.updateComponentHealthChecksWithFilter(componentHealthName, checks, func(string, healthv1.CheckResult) bool {
		return true
	})
}

// updateComponentHealthChecks replaces the entries named in ownedKeys with checks.
func (h *Handler) updateComponentHealthChecks(componentHealthName string, checks map[string]healthv1.CheckResult, ownedKeys []string) error {
	ownedKeySet := make(map[string]struct{}, len(ownedKeys))
	for _, key := range ownedKeys {
		ownedKeySet[key] = struct{}{}
	}
	return h.updateComponentHealthChecksWithFilter(componentHealthName, checks, func(key string, _ healthv1.CheckResult) bool {
		_, ok := ownedKeySet[key]
		return ok
	})
}

// updateComponentHealthChecksWithFilter replaces the existing entries matched by ownedCheck with checks,
// so one check function never clobbers another check function's entries within the same ComponentHealth.
// It skips the write entirely if the resulting checks are unchanged, which is what makes the
// reconcile loop converge.
func (h *Handler) updateComponentHealthChecksWithFilter(componentHealthName string, checks map[string]healthv1.CheckResult, ownedCheck func(string, healthv1.CheckResult) bool) error {
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
				LastCheckedAt: metav1.Now(),
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

	updatedChecks := make(map[string]healthv1.CheckResult, len(existing.Status.Checks)+len(checks))
	for key, check := range existing.Status.Checks {
		updatedChecks[key] = check
	}
	for key, check := range existing.Status.Checks {
		if ownedCheck(key, check) {
			delete(updatedChecks, key)
		}
	}
	for key, check := range checks {
		updatedChecks[key] = check
	}

	// the ignore marks are applied on the merged checks, so the checks kept from the other check functions
	// of the same ComponentHealth are marked too
	rules, err := h.listIgnoreRules(componentHealthName)
	if err != nil {
		return fmt.Errorf("failed to list HealthIgnoreRules: %w", err)
	}
	updatedChecks, nextExpiry := applyIgnoreRules(updatedChecks, rules, h.minSeverity, time.Now())
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
	toUpdate.Status.LastCheckedAt = metav1.Now()
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
