// SPDX-FileCopyrightText: © 2025 DSLab - Fondazione Bruno Kessler
//
// SPDX-License-Identifier: AGPL-3.0-or-later

package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// UserSpec defines the desired state of User
type UserSpec struct {
	// +kubebuilder:validation:Required
	AccessKey string `json:"accessKey"`
	// +kubebuilder:validation:Required
	SecretKey string `json:"secretKey"`
	// +kubebuilder:validation:Optional
	Policies []string `json:"policies,omitempty"`
	// +kubebuilder:validation:Optional
	// +kubebuilder:validation:Enum=enabled;disabled
	// +kubebuilder:default:=enabled
	AccountStatus string `json:"accountStatus,omitempty"`
}

// UserStatus defines the observed state of User
type UserStatus struct {
	// +operator-sdk:csv:customresourcedefinitions:type=status
	State   string `json:"state,omitempty" patchStrategy:"merge"`
	Message string `json:"message,omitempty" patchStrategy:"merge"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// User is the Schema for the users API
type User struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   UserSpec   `json:"spec,omitempty"`
	Status UserStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// UserList contains a list of User
type UserList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []User `json:"items"`
}

func init() {
	SchemeBuilder.Register(&User{}, &UserList{})
}
