package v1beta1

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

const (
	// LabelKeyIgnoreRuleSource tells who created a HealthIgnoreRule.
	LabelKeyIgnoreRuleSource = "health.harvesterhci.io/source"
	// IgnoreRuleSourceUser marks the rules created by users, e.g. from the dashboard.
	IgnoreRuleSourceUser = "user"
)

// +genclient
// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:resource:scope=Cluster

// HealthIgnoreRule tells the controller to mark the matching checks of a ComponentHealth as ignored.
// The checks are kept in the ComponentHealth status, so the ignore decision can be reverted and audited,
// but they no longer count in the HealthSummary.
type HealthIgnoreRule struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec HealthIgnoreRuleSpec `json:"spec,omitempty"`
}

type HealthIgnoreRuleSpec struct {
	// ComponentHealth is the name of the ComponentHealth that owns the check, e.g. harvester-controller-volume.
	// +kubebuilder:validation:MinLength=1
	ComponentHealth string `json:"componentHealth"`
	// CheckKey is the key of the check in the ComponentHealth status.
	// +kubebuilder:validation:MinLength=1
	CheckKey string `json:"checkKey"`
	// Resources limits the rule to these affected resources of the check, which are named the same as
	// status.checks[].affectedResources.names. The whole check is ignored when it is empty.
	// +optional
	Resources []string `json:"resources,omitempty"`
	// ExpiresAt is the time the rule stops taking effect. The rule never expires when it is not set.
	// +optional
	ExpiresAt *metav1.Time `json:"expiresAt,omitempty"`
	// Reason tells why the check is ignored.
	// +optional
	Reason string `json:"reason,omitempty"`
}
