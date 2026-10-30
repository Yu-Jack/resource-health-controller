package harvesterhci

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clienttesting "k8s.io/client-go/testing"

	harvesterv1 "github.com/harvester/harvester/pkg/apis/harvesterhci.io/v1beta1"
	healthv1 "github.com/harvester/resource-health-controller/pkg/apis/health.harvesterhci.io/v1beta1"
	"github.com/harvester/resource-health-controller/pkg/controller/componenthealth/healthcheck"
	"github.com/harvester/resource-health-controller/pkg/controller/componenthealth/testutil"
	"github.com/harvester/resource-health-controller/pkg/util/fakeclients"
)

func TestReconcileSettings(t *testing.T) {
	tests := []struct {
		name       string
		conditions []harvesterv1.Condition
		deleting   bool
		failed     bool
	}{
		{name: "setting without status"},
		{name: "configured", conditions: []harvesterv1.Condition{
			{Type: harvesterv1.SettingConfigured, Status: corev1.ConditionTrue},
		}},
		{name: "deleting setting is excluded", deleting: true, conditions: []harvesterv1.Condition{
			{Type: harvesterv1.SettingConfigured, Status: corev1.ConditionFalse, Message: "invalid value"},
		}},
		{name: "not configured", failed: true, conditions: []harvesterv1.Condition{
			{Type: harvesterv1.SettingConfigured, Status: corev1.ConditionFalse, Reason: "Failed", Message: "invalid value"},
		}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			setting := &harvesterv1.Setting{
				ObjectMeta: metav1.ObjectMeta{Name: "http-proxy"},
				Value:      `{"httpProxy":"http://user:secret@proxy:3128"}`,
				Customized: true,
				Status:     harvesterv1.SettingStatus{Conditions: tc.conditions},
			}
			if tc.deleting {
				deletedAt := metav1.Now()
				setting.DeletionTimestamp = &deletedAt
			}
			clientset, healthset := testutil.NewClients(setting)
			writer := healthcheck.Writer{Client: fakeclients.ComponentHealthClient(healthset.HealthV1beta1().ComponentHealths)}
			settings := &settingCheck{writer: writer, settingCache: fakeclients.SettingCache(clientset.HarvesterhciV1beta1().Settings)}
			if !assert.NoError(t, settings.Reconcile()) {
				return
			}
			health, err := healthset.HealthV1beta1().ComponentHealths().Get(context.Background(), settingComponentHealthName, metav1.GetOptions{})
			if !assert.NoError(t, err) {
				return
			}
			if !tc.failed {
				assert.Empty(t, health.Status.Checks)
				return
			}
			assert.Len(t, health.Status.Checks, 1)
			check := health.Status.Checks[checkKeySettingNotConfigured]
			assert.Equal(t, healthv1.SeverityError, check.Severity)
			assert.Equal(t, 1, check.AffectedCount)
			assert.Equal(t, settingKind, check.AffectedResources.Kind)
			assert.Equal(t, harvesterv1.SchemeGroupVersion.String(), check.AffectedResources.APIVersion)
			assert.Contains(t, check.AffectedResources.Names, "http-proxy")
			assert.Contains(t, check.Message, `reason="Failed", message="invalid value"`)
			assert.NotContains(t, check.Message, "secret", "the setting value must not be exposed")
		})
	}
}

func TestSettingChecksClearAndPreserveResources(t *testing.T) {
	ctx := context.Background()
	failed := []harvesterv1.Condition{{Type: harvesterv1.SettingConfigured, Status: corev1.ConditionFalse, Message: "invalid value"}}
	setting := &harvesterv1.Setting{
		ObjectMeta: metav1.ObjectMeta{Name: "http-proxy"},
		Status:     harvesterv1.SettingStatus{Conditions: failed},
	}
	other := setting.DeepCopy()
	other.Name = "storage-network"
	clientset, healthset := testutil.NewClients(setting, other)
	controller := &testutil.RecordingController{}
	writer := healthcheck.Writer{
		Controller: controller,
		Client:     fakeclients.ComponentHealthClient(healthset.HealthV1beta1().ComponentHealths),
	}
	settings := &settingCheck{writer: writer, settingCache: fakeclients.SettingCache(clientset.HarvesterhciV1beta1().Settings)}
	result, err := settings.OnSettingChanged("http-proxy", setting)
	assert.Same(t, setting, result)
	assert.NoError(t, err)
	assert.Equal(t, []string{settingComponentHealthName}, controller.Enqueued)
	if !assert.NoError(t, settings.Reconcile()) {
		return
	}
	health, err := healthset.HealthV1beta1().ComponentHealths().Get(ctx, settingComponentHealthName, metav1.GetOptions{})
	if !assert.NoError(t, err) {
		return
	}
	assert.Equal(t, 2, health.Status.Checks[checkKeySettingNotConfigured].AffectedCount)

	setting.Status.Conditions = []harvesterv1.Condition{{Type: harvesterv1.SettingConfigured, Status: corev1.ConditionTrue}}
	if _, err := clientset.HarvesterhciV1beta1().Settings().Update(ctx, setting, metav1.UpdateOptions{}); !assert.NoError(t, err) {
		return
	}
	if !assert.NoError(t, settings.Reconcile()) {
		return
	}
	health, err = healthset.HealthV1beta1().ComponentHealths().Get(ctx, settingComponentHealthName, metav1.GetOptions{})
	if !assert.NoError(t, err) {
		return
	}
	check := health.Status.Checks[checkKeySettingNotConfigured]
	assert.Equal(t, 1, check.AffectedCount)
	assert.NotContains(t, check.AffectedResources.Names, "http-proxy")

	if !assert.NoError(t, clientset.HarvesterhciV1beta1().Settings().Delete(ctx, other.Name, metav1.DeleteOptions{})) {
		return
	}
	result, err = settings.OnSettingChanged("storage-network", nil)
	assert.Nil(t, result)
	assert.NoError(t, err)
	if !assert.NoError(t, settings.Reconcile()) {
		return
	}
	health, err = healthset.HealthV1beta1().ComponentHealths().Get(ctx, settingComponentHealthName, metav1.GetOptions{})
	if assert.NoError(t, err) {
		assert.Empty(t, health.Status.Checks)
	}
}

func TestReconcileSettingsListError(t *testing.T) {
	clientset, _ := testutil.NewClients()
	clientset.PrependReactor("list", "settings", func(clienttesting.Action) (bool, runtime.Object, error) {
		return true, nil, fmt.Errorf("cache failure")
	})
	settings := &settingCheck{settingCache: fakeclients.SettingCache(clientset.HarvesterhciV1beta1().Settings)}
	assert.EqualError(t, settings.Reconcile(), "failed to list Settings: cache failure")
}
