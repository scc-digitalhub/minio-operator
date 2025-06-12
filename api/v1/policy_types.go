// SPDX-FileCopyrightText: © 2025 DSLab - Fondazione Bruno Kessler
//
// SPDX-License-Identifier: AGPL-3.0-or-later

package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// PolicySpec defines the desired state of Policy
type PolicySpec struct {
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Pattern:=`[^\s]*`
	Name string `json:"name"`
	// +kubebuilder:validation:Required
	Content string `json:"content"`
}

// PolicyStatus defines the observed state of Policy
type PolicyStatus struct {
	// +operator-sdk:csv:customresourcedefinitions:type=status
	State   string `json:"state,omitempty" patchStrategy:"merge"`
	Message string `json:"message,omitempty" patchStrategy:"merge"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// Policy is the Schema for the policies API
type Policy struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   PolicySpec   `json:"spec,omitempty"`
	Status PolicyStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// PolicyList contains a list of Policy
type PolicyList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Policy `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Policy{}, &PolicyList{})
}
