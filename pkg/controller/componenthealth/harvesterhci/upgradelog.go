package harvesterhci

import (
	"context"
	"fmt"

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
	upgradeLogControllerName      = "componentHealthUpgradeLogController"
	upgradeLogComponentHealthName = "harvester-controller-upgradelog"
	upgradeLogKind                = "UpgradeLog"

	checkKeyUpgradeLogOperatorNotReady = "UpgradeLogOperatorNotReady"
	checkKeyUpgradeLogInfraNotReady    = "UpgradeLogInfraNotReady"
	checkKeyUpgradeLogNotStarted       = "UpgradeLogNotStarted"
)

// upgradeLogFailureConditions maps the conditions of an UpgradeLog to the check raised while the condition is false.
// The logging infrastructure only collects logs, so a failure never blocks the upgrade and is a warning.
// DownloadReady is not listed because it is false whenever no archive is available to download.
var upgradeLogFailureConditions = []struct {
	cond condition.Cond
	key  string
}{
	{harvesterv1.LoggingOperatorDeployed, checkKeyUpgradeLogOperatorNotReady},
	{harvesterv1.InfraReady, checkKeyUpgradeLogInfraNotReady},
	{harvesterv1.UpgradeLogReady, checkKeyUpgradeLogNotStarted},
}

type upgradeLogCheck struct {
	writer          healthcheck.Writer
	upgradeLogCache ctlharvesterv1.UpgradeLogCache
}

// NewUpgradeLogCheck builds the check that reports UpgradeLogs whose logging infrastructure failed.
func NewUpgradeLogCheck(writer healthcheck.Writer, upgradeLogCache ctlharvesterv1.UpgradeLogCache) healthcheck.Check {
	return &upgradeLogCheck{writer: writer, upgradeLogCache: upgradeLogCache}
}

func (c *upgradeLogCheck) Name() string { return upgradeLogComponentHealthName }

func (c *upgradeLogCheck) OnUpgradeLogChanged(_ string, upgradeLog *harvesterv1.UpgradeLog) (*harvesterv1.UpgradeLog, error) {
	c.writer.Enqueue(upgradeLogComponentHealthName)
	return upgradeLog, nil
}

// RegisterUpgradeLogCheck builds the UpgradeLog check with its cache and registers its change handler.
func RegisterUpgradeLogCheck(ctx context.Context, writer healthcheck.Writer, management *config.Management) (healthcheck.Check, cache.InformerSynced) {
	upgradeLogs := management.HarvesterFactory.Harvesterhci().V1beta1().UpgradeLog()
	check := &upgradeLogCheck{writer: writer, upgradeLogCache: upgradeLogs.Cache()}
	upgradeLogs.OnChange(ctx, upgradeLogControllerName, check.OnUpgradeLogChanged)
	return check, upgradeLogs.Informer().HasSynced
}

func (c *upgradeLogCheck) Reconcile() error {
	upgradeLogs, err := c.upgradeLogCache.List(corev1.NamespaceAll, labels.Everything())
	if err != nil {
		return fmt.Errorf("failed to list UpgradeLogs: %w", err)
	}

	groups := healthcheck.ResourceGroups{}
	for _, upgradeLog := range upgradeLogs {
		if upgradeLog.DeletionTimestamp != nil {
			continue
		}
		name := healthcheck.NamespacedName(upgradeLog.Namespace, upgradeLog.Name)
		for _, failure := range upgradeLogFailureConditions {
			if failure.cond.IsFalse(upgradeLog) {
				groups.Add(failure.key, name, upgradeLogMessage(upgradeLog, name, failure.cond))
			}
		}
	}

	checks := map[string]healthv1.CheckResult{}
	for key, group := range groups {
		checks[key] = healthcheck.NewCheckResult(healthcheck.CheckResultSpec{
			Severity:   healthv1.SeverityWarning,
			Summary:    fmt.Sprintf("%d UpgradeLog(s)", len(group.Names)),
			APIVersion: harvesterv1.SchemeGroupVersion.String(),
			Kind:       upgradeLogKind,
			Names:      group.Names,
			Messages:   healthcheck.SortedMessages(group.Messages),
		})
	}
	return c.writer.ReplaceComponentHealthChecks(upgradeLogComponentHealthName, checks)
}

func upgradeLogMessage(upgradeLog *harvesterv1.UpgradeLog, name string, cond condition.Cond) string {
	return fmt.Sprintf("%s: %s=%s, reason=%q, message=%q, upgrade=%q",
		name, cond, cond.GetStatus(upgradeLog), cond.GetReason(upgradeLog), cond.GetMessage(upgradeLog), upgradeLog.Spec.UpgradeName)
}
