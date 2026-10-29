package harvesterhci

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/rancher/wrangler/v3/pkg/condition"
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
	upgradeControllerName      = "componentHealthUpgradeController"
	upgradeComponentHealthName = "harvester-controller-upgrade"
	upgradeKind                = "Upgrade"

	checkKeyUpgradeFailed               = "UpgradeFailed"
	checkKeyUpgradeImageNotReady        = "UpgradeImageNotReady"
	checkKeyUpgradeRepoNotProvisioned   = "UpgradeRepoNotProvisioned"
	checkKeyUpgradeNodesPrepareFailed   = "UpgradeNodesPrepareFailed"
	checkKeyUpgradeNodesUpgradeFailed   = "UpgradeNodesUpgradeFailed"
	checkKeyUpgradeSystemServicesFailed = "UpgradeSystemServicesFailed"
	checkKeyUpgradeLogNotReady          = "UpgradeLogNotReady"

	upgradeNodeStateFailed = "Failed"
)

// upgradeFailureConditions maps every condition of an Upgrade to the check it raises while the condition is false.
// UpgradeCompleted is the overall result and is also false when any of the stages fail, so it is reported as its own key.
var upgradeFailureConditions = []struct {
	cond     condition.Cond
	key      string
	severity healthv1.Severity
}{
	{harvesterv1.UpgradeCompleted, checkKeyUpgradeFailed, healthv1.SeverityError},
	{harvesterv1.ImageReady, checkKeyUpgradeImageNotReady, healthv1.SeverityError},
	{harvesterv1.RepoProvisioned, checkKeyUpgradeRepoNotProvisioned, healthv1.SeverityError},
	{harvesterv1.NodesPrepared, checkKeyUpgradeNodesPrepareFailed, healthv1.SeverityError},
	{harvesterv1.NodesUpgraded, checkKeyUpgradeNodesUpgradeFailed, healthv1.SeverityError},
	{harvesterv1.SystemServicesUpgraded, checkKeyUpgradeSystemServicesFailed, healthv1.SeverityError},
	// the upgrade can continue without the logging infrastructure
	{harvesterv1.LogReady, checkKeyUpgradeLogNotReady, healthv1.SeverityWarning},
}

type upgradeCheck struct {
	writer       healthcheck.Writer
	upgradeCache ctlharvesterv1.UpgradeCache
}

// NewUpgradeCheck builds the check that reports failed Upgrades.
func NewUpgradeCheck(writer healthcheck.Writer, upgradeCache ctlharvesterv1.UpgradeCache) healthcheck.Check {
	return &upgradeCheck{writer: writer, upgradeCache: upgradeCache}
}

func (c *upgradeCheck) Name() string { return upgradeComponentHealthName }

func (c *upgradeCheck) OnUpgradeChanged(_ string, upgrade *harvesterv1.Upgrade) (*harvesterv1.Upgrade, error) {
	c.writer.Enqueue(upgradeComponentHealthName)
	return upgrade, nil
}

// RegisterUpgradeCheck builds the Upgrade check with its cache and registers its change handler.
func RegisterUpgradeCheck(ctx context.Context, writer healthcheck.Writer, management *config.Management) (healthcheck.Check, cache.InformerSynced) {
	upgrades := management.HarvesterFactory.Harvesterhci().V1beta1().Upgrade()
	check := &upgradeCheck{writer: writer, upgradeCache: upgrades.Cache()}
	upgrades.OnChange(ctx, upgradeControllerName, check.OnUpgradeChanged)
	return check, upgrades.Informer().HasSynced
}

func (c *upgradeCheck) Reconcile() error {
	upgrades, err := c.upgradeCache.List(corev1.NamespaceAll, labels.Everything())
	if err != nil {
		return fmt.Errorf("failed to list Upgrades: %w", err)
	}

	groups := healthcheck.ResourceGroups{}
	severities := map[string]healthv1.Severity{}
	for _, upgrade := range upgrades {
		if upgrade.DeletionTimestamp != nil {
			continue
		}
		name := healthcheck.NamespacedName(upgrade.Namespace, upgrade.Name)
		for _, failure := range upgradeFailureConditions {
			if failure.cond.IsFalse(upgrade) {
				groups.Add(failure.key, name, upgradeMessage(upgrade, name, failure.cond))
				severities[failure.key] = failure.severity
			}
		}
	}

	checks := map[string]healthv1.CheckResult{}
	for key, group := range groups {
		checks[key] = healthcheck.NewCheckResult(healthcheck.CheckResultSpec{
			Severity:   severities[key],
			Summary:    fmt.Sprintf("%d Upgrade(s)", len(group.Names)),
			APIVersion: harvesterv1.SchemeGroupVersion.String(),
			Kind:       upgradeKind,
			Names:      group.Names,
			Messages:   healthcheck.SortedMessages(group.Messages),
		})
	}
	return c.writer.ReplaceComponentHealthChecks(upgradeComponentHealthName, checks)
}

func upgradeMessage(upgrade *harvesterv1.Upgrade, name string, cond condition.Cond) string {
	message := fmt.Sprintf("%s: %s=%s, reason=%q, message=%q, previousVersion=%q, version=%q",
		name, cond, cond.GetStatus(upgrade), cond.GetReason(upgrade), cond.GetMessage(upgrade),
		upgrade.Status.PreviousVersion, upgrade.Spec.Version)
	if cond == harvesterv1.NodesUpgraded {
		if failed := failedUpgradeNodes(upgrade); failed != "" {
			message += ", " + failed
		}
	}
	return message
}

// failedUpgradeNodes lists the nodes whose upgrade failed, in node name order.
func failedUpgradeNodes(upgrade *harvesterv1.Upgrade) string {
	nodeNames := make([]string, 0, len(upgrade.Status.NodeStatuses))
	for nodeName, status := range upgrade.Status.NodeStatuses {
		if status.State == upgradeNodeStateFailed {
			nodeNames = append(nodeNames, nodeName)
		}
	}
	sort.Strings(nodeNames)

	details := make([]string, 0, len(nodeNames))
	for _, nodeName := range nodeNames {
		status := upgrade.Status.NodeStatuses[nodeName]
		details = append(details, fmt.Sprintf("node %s: reason=%q, message=%q", nodeName, status.Reason, status.Message))
	}
	return strings.Join(details, "; ")
}
