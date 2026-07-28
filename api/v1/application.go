package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type WebAppList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`

	Items []WebApp `json:"items"`
}

type WebApp struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec WebAppSpec `json:"spec"`
}

type WebAppSpec struct {
	Image string              `json:"image"`
	Ports []WebAppPortMapping `json:"ports"`
}

type WebAppPortMapping struct {
	Internal int64 `json:"internal"`
	External int64 `json:"external"`
}
