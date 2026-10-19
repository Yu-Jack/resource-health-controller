package componenthealth

import (
	"k8s.io/apimachinery/pkg/runtime"

	harvfake "github.com/harvester/harvester/pkg/generated/clientset/versioned/fake"
	healthv1 "github.com/harvester/resource-health-controller/pkg/apis/health.harvesterhci.io/v1beta1"
	healthfake "github.com/harvester/resource-health-controller/pkg/generated/clientset/versioned/fake"
)

// newTestClients splits objects between the harvester fake clientset (watched resources)
// and the addon fake clientset (ComponentHealth / HealthSummary).
func newTestClients(objs ...runtime.Object) (*harvfake.Clientset, *healthfake.Clientset) {
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
