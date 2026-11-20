package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ScaleTargetRef points at the workload whose replica count is managed.
type ScaleTargetRef struct {
	// Kind is the target kind. Only Deployment is supported today.
	// +kubebuilder:validation:Enum=Deployment
	Kind string `json:"kind"`

	// Name is the target workload in the same namespace as the ScalePlan.
	Name string `json:"name"`
}

// ScaleWindow raises the replica count for the duration after each cron fire.
type ScaleWindow struct {
	// Name identifies the window in status and events.
	// +kubebuilder:validation:MinLength=1
	Name string `json:"name"`

	// Schedule is a standard five field cron expression.
	// +kubebuilder:validation:MinLength=1
	Schedule string `json:"schedule"`

	// TimeZone is an IANA zone name used to evaluate Schedule. Defaults to UTC.
	// +kubebuilder:default=UTC
	// +optional
	TimeZone string `json:"timeZone,omitempty"`

	// Duration is how long the window stays open after each fire.
	Duration metav1.Duration `json:"duration"`

	// Replicas is the replica count applied while the window is open.
	// +kubebuilder:validation:Minimum=0
	Replicas int32 `json:"replicas"`
}

// ScalePlanSpec defines the desired scaling behaviour of one workload.
type ScalePlanSpec struct {
	TargetRef ScaleTargetRef `json:"targetRef"`

	// Windows are evaluated in order; the last matching window wins.
	// +kubebuilder:validation:MaxItems=32
	// +optional
	Windows []ScaleWindow `json:"windows,omitempty"`

	// DefaultReplicas is applied when no window is open. When unset the
	// controller leaves the current replica count untouched.
	// +kubebuilder:validation:Minimum=0
	// +optional
	DefaultReplicas *int32 `json:"defaultReplicas,omitempty"`
}

// ScalePlanStatus reports which window the controller last applied.
type ScalePlanStatus struct {
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// ActiveWindow is the window currently applied, empty when none is open.
	// +optional
	ActiveWindow string `json:"activeWindow,omitempty"`

	// Replicas is the replica count the controller last wrote to the target.
	// +optional
	Replicas int32 `json:"replicas,omitempty"`

	// +optional
	// +listType=map
	// +listMapKey=type
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Target",type=string,JSONPath=`.spec.targetRef.name`
// +kubebuilder:printcolumn:name="Active",type=string,JSONPath=`.status.activeWindow`
// +kubebuilder:printcolumn:name="Replicas",type=integer,JSONPath=`.status.replicas`

// ScalePlan is the Schema for the scaleplans API.
type ScalePlan struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ScalePlanSpec   `json:"spec,omitempty"`
	Status ScalePlanStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// ScalePlanList contains a list of ScalePlan.
type ScalePlanList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ScalePlan `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ScalePlan{}, &ScalePlanList{})
}
