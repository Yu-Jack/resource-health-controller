// Package testutil holds the fakes shared by the tests of the ComponentHealth checks.
package testutil

import (
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

	harvfake "github.com/harvester/harvester/pkg/generated/clientset/versioned/fake"
	healthv1 "github.com/harvester/resource-health-controller/pkg/apis/health.harvesterhci.io/v1beta1"
	healthfake "github.com/harvester/resource-health-controller/pkg/generated/clientset/versioned/fake"
	ctlhealthv1 "github.com/harvester/resource-health-controller/pkg/generated/controllers/health.harvesterhci.io/v1beta1"
)

// RecordingController records the ComponentHealth keys that are enqueued.
type RecordingController struct {
	ctlhealthv1.ComponentHealthController
	Enqueued      []string
	EnqueuedAfter []string
}

func (c *RecordingController) EnqueueAfter(name string, _ time.Duration) {
	c.EnqueuedAfter = append(c.EnqueuedAfter, name)
}

func (c *RecordingController) Enqueue(name string) {
	c.Enqueued = append(c.Enqueued, name)
}

// NewIgnoreRule builds a HealthIgnoreRule that targets the given ComponentHealth and check key.
func NewIgnoreRule(componentHealth, name, checkKey string, resources []string, expiresAt *time.Time) *healthv1.HealthIgnoreRule {
	rule := &healthv1.HealthIgnoreRule{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec: healthv1.HealthIgnoreRuleSpec{
			ComponentHealth: componentHealth,
			CheckKey:        checkKey,
			Resources:       resources,
		},
	}
	if expiresAt != nil {
		rule.Spec.ExpiresAt = &metav1.Time{Time: *expiresAt}
	}
	return rule
}

// NewClients splits objects between the harvester fake clientset (watched resources)
// and the addon fake clientset (ComponentHealth / HealthSummary).
func NewClients(objs ...runtime.Object) (*harvfake.Clientset, *healthfake.Clientset) {
	var harvObjs, healthObjs []runtime.Object
	for _, obj := range objs {
		switch obj.(type) {
		case *healthv1.ComponentHealth, *healthv1.HealthSummary, *healthv1.HealthIgnoreRule:
			healthObjs = append(healthObjs, obj)
		default:
			harvObjs = append(harvObjs, obj)
		}
	}
	return harvfake.NewSimpleClientset(harvObjs...), healthfake.NewSimpleClientset(healthObjs...)
}
