package harvesterhci

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/tools/cache"

	harvesterv1 "github.com/harvester/harvester/pkg/apis/harvesterhci.io/v1beta1"
	ctlharvesterv1 "github.com/harvester/harvester/pkg/generated/controllers/harvesterhci.io/v1beta1"
	healthv1 "github.com/harvester/resource-health-controller/pkg/apis/health.harvesterhci.io/v1beta1"
	"github.com/harvester/resource-health-controller/pkg/config"
	"github.com/harvester/resource-health-controller/pkg/controller/componenthealth/healthcheck"
)

const (
	addonControllerName      = "componentHealthAddonController"
	addonComponentHealthName = "harvester-controller-addon"
	addonKind                = "Addon"

	checkKeyAddonOperationFailed = "AddonOperationFailed"
)

type addonCheck struct {
	writer     healthcheck.Writer
	addonCache ctlharvesterv1.AddonCache
}

// NewAddonCheck builds the check that reports Addons whose last operation failed.
func NewAddonCheck(writer healthcheck.Writer, addonCache ctlharvesterv1.AddonCache) healthcheck.Check {
	return &addonCheck{writer: writer, addonCache: addonCache}
}

func (c *addonCheck) Name() string { return addonComponentHealthName }

func (c *addonCheck) OnAddonChanged(_ string, addon *harvesterv1.Addon) (*harvesterv1.Addon, error) {
	c.writer.Enqueue(addonComponentHealthName)
	return addon, nil
}

// RegisterAddonCheck builds the Addon check with its cache and registers its change handler.
func RegisterAddonCheck(ctx context.Context, writer healthcheck.Writer, management *config.Management) (healthcheck.Check, cache.InformerSynced) {
	addons := management.HarvesterFactory.Harvesterhci().V1beta1().Addon()
	check := &addonCheck{writer: writer, addonCache: addons.Cache()}
	addons.OnChange(ctx, addonControllerName, check.OnAddonChanged)
	return check, addons.Informer().HasSynced
}

func (c *addonCheck) Reconcile() error {
	addons, err := c.addonCache.List(corev1.NamespaceAll, labels.Everything())
	if err != nil {
		return fmt.Errorf("failed to list Addons: %w", err)
	}

	groups := healthcheck.ResourceGroups{}
	for _, addon := range addons {
		if addon.DeletionTimestamp != nil {
			continue
		}
		// the addon controller sets OperationFailed when an enable, disable or update job fails or times out
		if harvesterv1.AddonOperationFailed.IsTrue(addon) {
			name := healthcheck.NamespacedName(addon.Namespace, addon.Name)
			groups.Add(checkKeyAddonOperationFailed, name, addonMessage(addon, name))
		}
	}

	checks := map[string]healthv1.CheckResult{}
	for key, group := range groups {
		checks[key] = healthcheck.NewCheckResult(healthcheck.CheckResultSpec{
			Severity:   healthv1.SeverityError,
			Summary:    fmt.Sprintf("%d Addon(s)", len(group.Names)),
			APIVersion: harvesterv1.SchemeGroupVersion.String(),
			Kind:       addonKind,
			Names:      group.Names,
			Messages:   healthcheck.SortedMessages(group.Messages),
		})
	}
	return c.writer.ReplaceComponentHealthChecks(addonComponentHealthName, checks)
}

func addonMessage(addon *harvesterv1.Addon, name string) string {
	cond := harvesterv1.AddonOperationFailed
	return fmt.Sprintf("%s: state=%q, enabled=%t, %s=%s, reason=%q, message=%q",
		name, addon.Status.Status, addon.Spec.Enabled, cond, cond.GetStatus(addon), cond.GetReason(addon), cond.GetMessage(addon))
}
