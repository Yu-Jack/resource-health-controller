package v1beta1

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

// +genclient
// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:resource:scope=Cluster

// HealthSummary aggregates all ComponentHealth resources into a single cluster-wide overview.
// There is exactly one HealthSummary resource, named "cluster".
type HealthSummary struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Status HealthSummaryStatus `json:"status,omitempty"`
}

type HealthSummaryStatus struct {
	// +optional
	LastCheckedAt metav1.Time `json:"lastCheckedAt,omitempty"`
	// +optional
	Components map[string]ComponentSummary `json:"components,omitempty"`
}

type ComponentSummary struct {
	ErrorCount   int `json:"errorCount"`
	WarningCount int `json:"warningCount"`
}
