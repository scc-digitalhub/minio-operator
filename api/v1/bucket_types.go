/*
Copyright 2024.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// BucketSpec defines the desired state of Bucket
type BucketSpec struct {
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Pattern:=`^[a-z0-9]([a-z0-9\.-]){1,61}[a-z0-9]$`
	Name string `json:"name"`
	// +kubebuilder:validation:Optional
	Quota uint64 `json:"quota,omitempty"`
}

// BucketStatus defines the observed state of Bucket
type BucketStatus struct {
	// +operator-sdk:csv:customresourcedefinitions:type=status
	State   string `json:"state,omitempty" patchStrategy:"merge"`
	Message string `json:"message,omitempty" patchStrategy:"merge"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// Bucket is the Schema for the buckets API
type Bucket struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   BucketSpec   `json:"spec,omitempty"`
	Status BucketStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// BucketList contains a list of Bucket
type BucketList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Bucket `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Bucket{}, &BucketList{})
}
