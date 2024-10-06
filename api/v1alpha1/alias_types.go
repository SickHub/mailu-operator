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

// AliasSpec defines the desired state of Alias
type AliasSpec struct {
	// Name part of e-mail address 'name@domain'.
	Name string `json:"name"`
	// Domain part of e-mail address 'name@domain'.
	Domain string `json:"domain"`
	// Comment is a custom comment for the alias.
	Comment string `json:"comment,omitempty"`
	// Destination is a list of destinations for e-mails to 'name@domain'.
	Destination []string `json:"destination,omitempty"`
	// Wildcard must be set to 'true' if the name contains the wildcard character '%'.
	Wildcard bool `json:"wildcard,omitempty"`
}

// AliasStatus defines the observed state of Alias
type AliasStatus struct {
	// +operator-sdk:csv:customresourcedefinitions:type=status
	Conditions []metav1.Condition `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// Alias is the Schema for the aliases API
type Alias struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   AliasSpec   `json:"spec,omitempty"`
	Status AliasStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// AliasList contains a list of Alias
type AliasList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Alias `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Alias{}, &AliasList{})
}
