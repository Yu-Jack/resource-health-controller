package componenthealth

import (
	"context"
	"fmt"

	"k8s.io/client-go/tools/cache"

	healthv1 "github.com/harvester/resource-health-controller/pkg/apis/health.harvesterhci.io/v1beta1"
	"github.com/harvester/resource-health-controller/pkg/config"
	ctlhealthv1 "github.com/harvester/resource-health-controller/pkg/generated/controllers/health.harvesterhci.io/v1beta1"
)

// healthCheck is a unit that owns one ComponentHealth and recomputes all of its checks.
type healthCheck interface {
	// name is the name of the ComponentHealth the check owns.
	name() string
	reconcile() error
}

// checkRegistration builds a check with its caches, registers its change handlers and returns its HasSynced.
type checkRegistration func(ctx context.Context, writer componentHealthWriter, management *config.Management) (healthCheck, cache.InformerSynced)

// checkRegistrations is the registry of checks; a new check only needs to be added here.
var checkRegistrations = []checkRegistration{
	registerNodeCheck,
	registerVMICheck,
	registerVolumeCheck,
	registerVMBackupCheck,
	registerScheduleVMBackupCheck,
	registerVMImageCheck,
}

// Handler dispatches ComponentHealth keys to the check that owns them.
type Handler struct {
	componentHealthController ctlhealthv1.ComponentHealthController
	healthCachesSynced        []cache.InformerSynced
	checks                    []healthCheck
}

func Register(ctx context.Context, management *config.Management) error {
	componentHealths := management.HealthFactory.Health().V1beta1().ComponentHealth()
	ignoreRules := management.HealthFactory.Health().V1beta1().HealthIgnoreRule()

	writer := componentHealthWriter{
		controller:  componentHealths,
		client:      componentHealths,
		ignoreRules: ignoreRules.Cache(),
	}
	h := &Handler{
		componentHealthController: componentHealths,
		healthCachesSynced:        []cache.InformerSynced{ignoreRules.Informer().HasSynced},
	}

	// resource handlers only enqueue the ComponentHealth key, the workqueue de-duplicates them
	for _, register := range checkRegistrations {
		check, synced := register(ctx, writer, management)
		h.checks = append(h.checks, check)
		h.healthCachesSynced = append(h.healthCachesSynced, synced)
	}
	ignoreRules.OnChange(ctx, ignoreRuleControllerName, h.OnIgnoreRuleChanged)

	// the only place checks are computed: dispatch by ComponentHealth key, registered once h.checks is complete
	componentHealths.OnChange(ctx, "component-health-reconcile", h.OnComponentHealthChanged)

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
	for _, check := range h.checks {
		h.enqueueHealth(check.name())
	}
}

func (h *Handler) OnComponentHealthChanged(key string, health *healthv1.ComponentHealth) (*healthv1.ComponentHealth, error) {
	for _, check := range h.checks {
		if check.name() != key {
			continue
		}
		for _, synced := range h.healthCachesSynced {
			if !synced() {
				return health, fmt.Errorf("ComponentHealth resource caches are not synced")
			}
		}
		return health, check.reconcile()
	}
	return health, nil
}
