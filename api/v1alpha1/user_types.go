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

// UserSpec defines the desired state of User
type UserSpec struct {
	AllowSpoofing      bool   `json:"allowSpoofing,omitempty"`
	ChangePassword     bool   `json:"changePassword,omitempty"`
	Comment            string `json:"comment,omitempty"`
	DisplayedName      string `json:"displayedName,omitempty"`
	Domain             string `json:"domain"`
	Enabled            bool   `json:"enabled,omitempty"`
	EnableIMAP         bool   `json:"enableIMAP,omitempty"`
	EnablePOP          bool   `json:"enablePOP,omitempty"`
	ForwardEnabled     bool   `json:"forwardEnabled,omitempty"`
	ForwardDestination string `json:"forwardDestination,omitempty"`
	ForwardKeep        bool   `json:"forwardKeep,omitempty"`
	GlobalAdmin        bool   `json:"globalAdmin,omitempty"`
	Name               string `json:"name"`
	// TODO: rename?
	PasswordSecret string `json:"passwordSecret,omitempty"`
	PasswordKey    string `json:"passwordKey,omitempty"`
	QuotaBytes     int64  `json:"quotaBytes,omitempty"`
	ReplyEnabled   bool   `json:"replyEnabled,omitempty"`
	ReplySubject   string `json:"replySubject,omitempty"`
	ReplyBody      string `json:"replyBody,omitempty"`
	ReplyStartDate string `json:"replyStartDate,omitempty"`
	ReplyEndDate   string `json:"replyEndDate,omitempty"`
	SpamEnabled    bool   `json:"spamEnabled,omitempty"`
	SpamMarkAsRead bool   `json:"spamMarkAsRead,omitempty"`
	SpamThreshold  int64  `json:"spamThreshold,omitempty"`
}

// UserStatus defines the observed state of User
type UserStatus struct {
	// +operator-sdk:csv:customresourcedefinitions:type=status
	Conditions []metav1.Condition `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type" protobuf:"bytes,1,rep,name=conditions"`
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
