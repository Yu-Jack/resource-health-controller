package healthsummary

import (
	"context"
	"fmt"
	"reflect"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"

	healthv1 "github.com/harvester/resource-health-controller/pkg/apis/health.harvesterhci.io/v1beta1"
	"github.com/harvester/resource-health-controller/pkg/config"
	ctlhealthv1 "github.com/harvester/resource-health-controller/pkg/generated/controllers/health.harvesterhci.io/v1beta1"
)

const (
	summaryName           = "cluster"
	summaryControllerName = "healthSummaryReconciler"
)

type Handler struct {
	healthSummaries     ctlhealthv1.HealthSummaryClient
	componentHealthCtrl ctlhealthv1.ComponentHealthClient
}

func Register(ctx context.Context, management *config.Management) error {
	componentHealthCtrl := management.HealthFactory.Health().V1beta1().ComponentHealth()
	healthSummaries := management.HealthFactory.Health().V1beta1().HealthSummary()

	h := &Handler{
		healthSummaries:     healthSummaries,
		componentHealthCtrl: componentHealthCtrl,
	}

	componentHealthCtrl.OnChange(ctx, summaryControllerName, h.onComponentHealthChanged)
	return nil
}

// onComponentHealthChanged fires for every ComponentHealth event and reconciles HealthSummary/cluster.
func (h *Handler) onComponentHealthChanged(_ string, obj *healthv1.ComponentHealth) (*healthv1.ComponentHealth, error) {
	if obj == nil || obj.DeletionTimestamp != nil {
		return nil, nil
	}
	return obj, h.reconcileSummary()
}

func (h *Handler) reconcileSummary() error {
	allHealths, err := h.componentHealthCtrl.List(metav1.ListOptions{
		LabelSelector: labels.Everything().String(),
	})
	if err != nil {
		return fmt.Errorf("failed to list ComponentHealths: %w", err)
	}

	components := map[string]healthv1.ComponentSummary{}
	for _, ch := range allHealths.Items {
		key := ch.Name
		if component, ok := ch.Labels[healthv1.LabelKeyComponent]; ok && component != "" {
			key = component
		}
		entry := components[key]
		for _, check := range ch.Status.Checks {
			if check.Ignored {
				continue
			}
			switch check.Severity {
			case healthv1.SeverityError:
				entry.ErrorCount++
			case healthv1.SeverityWarning:
				entry.WarningCount++
			}
		}
		components[key] = entry
	}

	return h.updateHealthSummary(components)
}

func (h *Handler) updateHealthSummary(components map[string]healthv1.ComponentSummary) error {
	existing, err := h.healthSummaries.Get(summaryName, metav1.GetOptions{})
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return fmt.Errorf("failed to get HealthSummary %s: %w", summaryName, err)
		}
		_, err = h.healthSummaries.Create(&healthv1.HealthSummary{
			ObjectMeta: metav1.ObjectMeta{Name: summaryName},
			Status: healthv1.HealthSummaryStatus{
				LastCheckedAt: metav1.Now(),
				Components:    components,
			},
		})
		return err
	}

	if reflect.DeepEqual(existing.Status.Components, components) {
		return nil
	}

	toUpdate := existing.DeepCopy()
	toUpdate.Status.LastCheckedAt = metav1.Now()
	toUpdate.Status.Components = components
	_, err = h.healthSummaries.Update(toUpdate)
	return err
}
