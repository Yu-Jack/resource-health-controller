package healthcheck

import (
	"sort"
	"time"

	"k8s.io/apimachinery/pkg/labels"

	healthv1 "github.com/harvester/resource-health-controller/pkg/apis/health.harvesterhci.io/v1beta1"
	ctlhealthv1 "github.com/harvester/resource-health-controller/pkg/generated/controllers/health.harvesterhci.io/v1beta1"
)

// listIgnoreRules returns the rules that target the given ComponentHealth, sorted by name so the result is deterministic.
func listIgnoreRules(ignoreRules ctlhealthv1.HealthIgnoreRuleCache, componentHealthName string) ([]*healthv1.HealthIgnoreRule, error) {
	if ignoreRules == nil {
		return nil, nil
	}
	all, err := ignoreRules.List(labels.Everything())
	if err != nil {
		return nil, err
	}
	rules := make([]*healthv1.HealthIgnoreRule, 0, len(all))
	for _, rule := range all {
		if rule.Spec.ComponentHealth == componentHealthName {
			rules = append(rules, rule)
		}
	}
	sort.Slice(rules, func(i, j int) bool { return rules[i].Name < rules[j].Name })
	return rules, nil
}

// applyIgnoreRules returns a copy of checks in which the ignored checks and resources are marked.
//
// The marks are always derived from scratch: any mark carried by checks is cleared first, so a removed or
// expired rule un-ignores the check. The result only depends on its inputs (now is only compared against
// the rule expiry), which keeps the "unchanged, skip the write" comparison of the caller convergent.
//
// The returned expiry is the earliest time at which a rule that currently takes effect stops doing so,
// nil if there is none, the caller has to reconcile again at that time.
func applyIgnoreRules(checks map[string]healthv1.CheckResult, rules []*healthv1.HealthIgnoreRule, now time.Time) (map[string]healthv1.CheckResult, *time.Time) {
	var nextExpiry *time.Time
	result := make(map[string]healthv1.CheckResult, len(checks))

	for key, original := range checks {
		check := *original.DeepCopy()
		check.Ignored = false
		check.IgnoredBy = ""
		if check.AffectedResources != nil {
			for name := range check.AffectedResources.Names {
				check.AffectedResources.Names[name] = healthv1.AffectedResourceDetail{}
			}
		}

		var firstRule string
		for _, rule := range rules {
			if rule.Spec.CheckKey != key {
				continue
			}
			if expiresAt := rule.Spec.ExpiresAt; expiresAt != nil {
				if !expiresAt.After(now) {
					continue
				}
				if nextExpiry == nil || expiresAt.Time.Before(*nextExpiry) {
					t := expiresAt.Time
					nextExpiry = &t
				}
			}

			if len(rule.Spec.Resources) == 0 {
				check.Ignored = true
				check.IgnoredBy = rule.Name
				break
			}
			if check.AffectedResources == nil {
				continue
			}
			for _, name := range rule.Spec.Resources {
				if detail, ok := check.AffectedResources.Names[name]; ok && !detail.Ignored {
					check.AffectedResources.Names[name] = healthv1.AffectedResourceDetail{Ignored: true, IgnoredBy: rule.Name}
					if firstRule == "" {
						firstRule = rule.Name
					}
				}
			}
		}

		// a check is ignored as a whole once every affected resource of it is ignored
		if !check.Ignored && firstRule != "" && allResourcesIgnored(check.AffectedResources) {
			check.Ignored = true
			check.IgnoredBy = firstRule
		}
		result[key] = check
	}
	return result, nextExpiry
}

func allResourcesIgnored(resources *healthv1.AffectedResources) bool {
	if resources == nil || len(resources.Names) == 0 {
		return false
	}
	for _, detail := range resources.Names {
		if !detail.Ignored {
			return false
		}
	}
	return true
}
