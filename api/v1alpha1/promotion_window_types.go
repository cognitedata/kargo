package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +kubebuilder:resource:shortName={promotionwindow,promotionwindows}
// +kubebuilder:object:root=true
// +kubebuilder:printcolumn:name=TimeZone,type=string,JSONPath=`.spec.timeZone`

// PromotionWindows defines time windows during which automatic promotions
// are allowed to occur. If not specified, automatic promotions can occur at
// any time.
type PromotionWindow struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty" protobuf:"bytes,1,opt,name=metadata"`

	// Spec describes the composition of an AutoPromotionWindow, including the
	// recurring time window and time zone.
	//
	// +kubebuilder:validation:Required
	Spec PromotionWindowSpec `json:"spec" protobuf:"bytes,2,opt,name=spec"`
}

type PromotionWindowSpec struct {
	// Kind is either "deny" or "allow", indicating whether the time window
	// represents a period during which promotions are denied or allowed.
	//
	// +kubebuilder:validation:Enum=deny;allow;
	Kind string `json:"kind,omitempty" protobuf:"bytes,1,opt,name=kind"`
	// Schedule describes a recurring time window.
	// Example: "0 0 * * 1-5" means every weekday at midnight.
	//
	// +kubebuilder:validation:Required
	Schedule string `json:"schedule" protobuf:"bytes,2,opt,name=schedule"`
	// It seems `metav1.Duration` doesn't imply any validation.
	// Also using kubebuilder's `validation:Minimum` doesn't do any thing here.
	// Also Also, from CEL point of view, this field is essentially a string.
	// The following CEL rule does the job. Feel free to rewrite this in a simpler format if you can.

	// Duration is the length of time that the window lasts after the start
	// time defined by the Schedule.
	//
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:XValidation:rule="duration(self) > duration('0s')", message="Duration must follow duration format (e.g: 30s) and be positive"
	Duration *metav1.Duration `json:"duration,omitempty" protobuf:"bytes,3,opt,name=duration"`
	// TimeZone is the IANA time zone name that applies to the time window.
	// If not specified, UTC is assumed.
	TimeZone string `json:"timeZone,omitempty" protobuf:"bytes,4,opt,name=timeZone"`

	// LabelSelector to target either Projects or Stages.
	// I am using a LabelSelector instead of a typed field to isolate all the changes to limit the conflict with upstream.
	// Beware: empty labelSelector means it matches all in the namespace
	// +optional
	LabelSelector metav1.LabelSelector `json:"labelSelector,omitempty" protobuf:"bytes,5,opt,name=labelSelector"`
}

// +kubebuilder:object:root=true

// PromotionWindowList contains a list of PromotionWindows.
type PromotionWindowList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty" protobuf:"bytes,1,opt,name=metadata"`
	Items           []PromotionWindow `json:"items" protobuf:"bytes,2,rep,name=items"`
}
