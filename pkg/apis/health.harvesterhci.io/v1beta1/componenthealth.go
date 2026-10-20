package v1beta1

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

type Severity string

const (
	SeverityError   Severity = "Error"
	SeverityWarning Severity = "Warning"
	SeverityInfo    Severity = "Info"

	// LabelKeyComponent groups all ComponentHealth CRs belonging to the same logical component.
	LabelKeyComponent = "health.harvesterhci.io/component"
)

// +genclient
// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:resource:scope=Cluster

// ComponentHealth reports the latest health check results of a single resource kind of a component.
type ComponentHealth struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Status ComponentHealthStatus `json:"status,omitempty"`
}

type ComponentHealthStatus struct {
	// +optional
	LastUpdatedAt metav1.Time `json:"lastUpdatedAt,omitempty"`
	// +optional
	Checks map[string]CheckResult `json:"checks,omitempty"`
}

type CheckResult struct {
	Severity Severity `json:"severity"`
	Message  string   `json:"message"`
	// +optional
	AffectedCount int `json:"affectedCount,omitempty"`
	// +optional
	AffectedResources *AffectedResources `json:"affectedResources,omitempty"`
	// Ignored is true when the whole check is ignored, it does not count in the HealthSummary then.
	// +optional
	Ignored bool `json:"ignored,omitempty"`
	// IgnoredBy is the name of the HealthIgnoreRule that ignores the check.
	// +optional
	IgnoredBy string `json:"ignoredBy,omitempty"`
}

type AffectedResources struct {
	APIVersion string `json:"apiVersion"`
	Kind       string `json:"kind"`
	// +optional
	Names map[string]AffectedResourceDetail `json:"names,omitempty"`
}

type AffectedResourceDetail struct {
	// Ignored is true when this resource is ignored by a HealthIgnoreRule.
	// +optional
	Ignored bool `json:"ignored,omitempty"`
	// IgnoredBy is the name of the HealthIgnoreRule that ignores the resource.
	// +optional
	IgnoredBy string `json:"ignoredBy,omitempty"`
}
