package componenthealth

import (
	healthv1 "github.com/harvester/resource-health-controller/pkg/apis/health.harvesterhci.io/v1beta1"
)

const ignoreRuleControllerName = "componentHealthIgnoreRuleController"

// OnIgnoreRuleChanged queues a reconciliation of every ComponentHealth on every HealthIgnoreRule change.
// A deleted rule is seen as nil and an updated rule may have switched ComponentHealth, so the affected
// ComponentHealth can not be told from the object. There are only a few keys and the workqueue de-duplicates them.
func (h *Handler) OnIgnoreRuleChanged(_ string, rule *healthv1.HealthIgnoreRule) (*healthv1.HealthIgnoreRule, error) {
	h.enqueueAllHealth()
	return rule, nil
}
