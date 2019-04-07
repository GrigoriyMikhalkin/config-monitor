package v1alpha1

import (
	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// MonitoredServiceSpec defines the desired state of MonitoredService
// +k8s:openapi-gen=true
type MonitoredServiceSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "operator-sdk generate k8s" to regenerate code after modifying this file
	// Add custom validation using kubebuilder tags: https://book.kubebuilder.io/beyond_basics/generating_crd.html
	Image		string	`json:"image"`
	Size 		int32 	`json:"size"`
	ConfigRepo	string	`json:"config_repo"`
}

// MonitoredServiceStatus defines the observed state of MonitoredService
// +k8s:openapi-gen=true
type MonitoredServiceStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "operator-sdk generate k8s" to regenerate code after modifying this file
	// Add custom validation using kubebuilder tags: https://book.kubebuilder.io/beyond_basics/generating_crd.html
	Nodes 		[]string 	`json:"nodes"`
	PodSpec 	v1.PodSpec	`json:"pod_spec"`
	SpecChanged	bool	`json:"spec_changed"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// MonitoredService is the Schema for the monitoredservices API
// +k8s:openapi-gen=true
type MonitoredService struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   MonitoredServiceSpec   `json:"spec,omitempty"`
	Status MonitoredServiceStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// MonitoredServiceList contains a list of MonitoredService
type MonitoredServiceList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []MonitoredService `json:"items"`
}

func init() {
	SchemeBuilder.Register(&MonitoredService{}, &MonitoredServiceList{})
}
