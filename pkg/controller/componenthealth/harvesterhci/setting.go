package harvesterhci

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/tools/cache"

	harvesterv1 "github.com/harvester/harvester/pkg/apis/harvesterhci.io/v1beta1"
	ctlharvesterv1 "github.com/harvester/harvester/pkg/generated/controllers/harvesterhci.io/v1beta1"
	healthv1 "github.com/harvester/resource-health-controller/pkg/apis/health.harvesterhci.io/v1beta1"
	"github.com/harvester/resource-health-controller/pkg/config"
	"github.com/harvester/resource-health-controller/pkg/controller/componenthealth/healthcheck"
)

const (
	settingControllerName      = "componentHealthSettingController"
	settingComponentHealthName = "harvester-controller-setting"
	settingKind                = "Setting"

	checkKeySettingNotConfigured = "SettingNotConfigured"
)

type settingCheck struct {
	writer       healthcheck.Writer
	settingCache ctlharvesterv1.SettingCache
}

// NewSettingCheck builds the check that reports Settings that could not be applied.
func NewSettingCheck(writer healthcheck.Writer, settingCache ctlharvesterv1.SettingCache) healthcheck.Check {
	return &settingCheck{writer: writer, settingCache: settingCache}
}

func (c *settingCheck) Name() string { return settingComponentHealthName }

func (c *settingCheck) OnSettingChanged(_ string, setting *harvesterv1.Setting) (*harvesterv1.Setting, error) {
	c.writer.Enqueue(settingComponentHealthName)
	return setting, nil
}

// RegisterSettingCheck builds the Setting check with its cache and registers its change handler.
func RegisterSettingCheck(ctx context.Context, writer healthcheck.Writer, management *config.Management) (healthcheck.Check, cache.InformerSynced) {
	settings := management.HarvesterFactory.Harvesterhci().V1beta1().Setting()
	check := &settingCheck{writer: writer, settingCache: settings.Cache()}
	settings.OnChange(ctx, settingControllerName, check.OnSettingChanged)
	return check, settings.Informer().HasSynced
}

func (c *settingCheck) Reconcile() error {
	settings, err := c.settingCache.List(labels.Everything())
	if err != nil {
		return fmt.Errorf("failed to list Settings: %w", err)
	}

	groups := healthcheck.ResourceGroups{}
	for _, setting := range settings {
		if setting.DeletionTimestamp != nil {
			continue
		}
		// the setting handlers set configured=false with the error that stopped them from applying the value
		if harvesterv1.SettingConfigured.IsFalse(setting) {
			groups.Add(checkKeySettingNotConfigured, setting.Name, settingMessage(setting))
		}
	}

	checks := map[string]healthv1.CheckResult{}
	for key, group := range groups {
		checks[key] = healthcheck.NewCheckResult(healthcheck.CheckResultSpec{
			Severity:   healthv1.SeverityError,
			Summary:    fmt.Sprintf("%d Setting(s)", len(group.Names)),
			APIVersion: harvesterv1.SchemeGroupVersion.String(),
			Kind:       settingKind,
			Names:      group.Names,
			Messages:   healthcheck.SortedMessages(group.Messages),
		})
	}
	return c.writer.ReplaceComponentHealthChecks(settingComponentHealthName, checks)
}

// settingMessage leaves the value out because settings such as http-proxy and ssl-certificates can hold credentials.
func settingMessage(setting *harvesterv1.Setting) string {
	cond := harvesterv1.SettingConfigured
	return fmt.Sprintf("%s: customized=%t, %s=%s, reason=%q, message=%q",
		setting.Name, setting.Customized, cond, cond.GetStatus(setting), cond.GetReason(setting), cond.GetMessage(setting))
}
