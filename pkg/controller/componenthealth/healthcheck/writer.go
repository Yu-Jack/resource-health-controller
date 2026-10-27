package healthcheck

import (
	"fmt"
	"reflect"
	"sort"
	"time"

	"github.com/sirupsen/logrus"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	healthv1 "github.com/harvester/resource-health-controller/pkg/apis/health.harvesterhci.io/v1beta1"
	ctlhealthv1 "github.com/harvester/resource-health-controller/pkg/generated/controllers/health.harvesterhci.io/v1beta1"
)

// ComponentName is the logical component name of the checks that watch harvester/harvester resources.
const ComponentName = "harvester-controller"

// Writer is what a check needs to publish its results.
type Writer struct {
	Controller  ctlhealthv1.ComponentHealthController
	Client      ctlhealthv1.ComponentHealthClient
	IgnoreRules ctlhealthv1.HealthIgnoreRuleCache
}

// Enqueue queues a reconciliation of the named ComponentHealth.
func (w Writer) Enqueue(name string) {
	w.Controller.Enqueue(name)
}

// ReplaceComponentHealthChecks replaces all the checks of a ComponentHealth, which is owned by a single
// reconcile function, so checks that no longer apply are removed as well.
// It skips the write entirely if the resulting checks are unchanged, which is what makes the
// reconcile loop converge.
func (w Writer) ReplaceComponentHealthChecks(componentHealthName string, checks map[string]healthv1.CheckResult) error {
	rules, err := listIgnoreRules(w.IgnoreRules, componentHealthName)
	if err != nil {
		return fmt.Errorf("failed to list HealthIgnoreRules: %w", err)
	}
	updatedChecks, nextExpiry := applyIgnoreRules(checks, rules, time.Now())
	if nextExpiry != nil {
		// nothing else triggers a reconcile when a rule expires
		w.Controller.EnqueueAfter(componentHealthName, time.Until(*nextExpiry))
	}

	existing, err := w.Client.Get(componentHealthName, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		_, err = w.Client.Create(&healthv1.ComponentHealth{
			ObjectMeta: metav1.ObjectMeta{
				Name: componentHealthName,
				Labels: map[string]string{
					healthv1.LabelKeyComponent: ComponentName,
				},
			},
			Status: healthv1.ComponentHealthStatus{
				LastUpdatedAt: metav1.Now(),
				Checks:        updatedChecks,
			},
		})
		if err != nil {
			return fmt.Errorf("failed to create ComponentHealth %s: %w", componentHealthName, err)
		}
		logrus.WithFields(logrus.Fields{
			"component":        ComponentName,
			"component_health": componentHealthName,
			"checks":           getCheckKeys(checks),
		}).Info("created ComponentHealth checks")
		return nil
	}
	if err != nil {
		return fmt.Errorf("failed to get ComponentHealth %s: %w", componentHealthName, err)
	}

	if len(existing.Status.Checks) == 0 && len(updatedChecks) == 0 || reflect.DeepEqual(existing.Status.Checks, updatedChecks) {
		logrus.WithFields(logrus.Fields{
			"component":        ComponentName,
			"component_health": componentHealthName,
			"checks":           getCheckKeys(checks),
		}).Debug("ComponentHealth checks unchanged")
		return nil
	}

	toUpdate := existing.DeepCopy()
	if toUpdate.Labels == nil {
		toUpdate.Labels = map[string]string{}
	}
	toUpdate.Labels[healthv1.LabelKeyComponent] = ComponentName
	toUpdate.Status.LastUpdatedAt = metav1.Now()
	toUpdate.Status.Checks = updatedChecks
	_, err = w.Client.Update(toUpdate)
	if err != nil {
		return err
	}
	logrus.WithFields(logrus.Fields{
		"component":        ComponentName,
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
