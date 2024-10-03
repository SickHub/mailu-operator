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

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// DomainSpec defines the desired state of Domain
type DomainSpec struct {
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	// Domain name.
	// TODO use a simplified regex to validate?
	Name    string `json:"name"`
	Comment string `json:"comment,omitempty"`
	// -1 ... x
	MaxUsers int `json:"maxUsers,omitempty"`
	// -1 ... x
	MaxAliases int `json:"maxAliases,omitempty"`
	// ?
	MaxQuotaBytes int      `json:"maxQuotaBytes,omitempty"`
	SignupEnabled bool     `json:"signupEnabled,omitempty"`
	Alternatives  []string `json:"alternatives,omitempty"`
}

// DomainStatus defines the observed state of Domain
type DomainStatus struct {
	// +operator-sdk:csv:customresourcedefinitions:type=status
	Conditions []metav1.Condition `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// Domain is the Schema for the domains API
type Domain struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   DomainSpec   `json:"spec,omitempty"`
	Status DomainStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// DomainList contains a list of Domain
type DomainList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Domain `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Domain{}, &DomainList{})
}
