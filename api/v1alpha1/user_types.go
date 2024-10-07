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
	// Name part of e-mail address 'name@domain'.
	Name string `json:"name"`
	// Domain part of e-mail address 'name@domain'.
	Domain string `json:"domain"`
	// AllowSpoofing allows this user to send e-mails with any sender.
	// +kubebuilder:default=false
	AllowSpoofing bool `json:"allowSpoofing,omitempty"`
	// ChangePassword requires the user to change the password on next login.
	// +kubebuilder:default=false
	ChangePassword bool `json:"changePassword,omitempty"`
	// Comment is a custom comment for the user.
	Comment string `json:"comment,omitempty"`
	// DisplayName is the name displayed for this user.
	// +kubebuilder:default=""
	DisplayedName string `json:"displayedName,omitempty"`
	// Enabled states the status of this user account.
	// +kubebuilder:default=false
	Enabled bool `json:"enabled,omitempty"`
	// EnableIMAP states if IMAP is available to the user.
	// +kubebuilder:default=false
	EnableIMAP bool `json:"enableIMAP,omitempty"`
	// EnablePOP states if POP3 is available to the user.
	// +kubebuilder:default=false
	EnablePOP bool `json:"enablePOP,omitempty"`
	// ForwardEnabled states if e-mails are forwarded.
	// +kubebuilder:default=false
	ForwardEnabled bool `json:"forwardEnabled,omitempty"`
	// ForwardDestination states the destination(s) to forward e-mail to.
	// +kubebuilder:default={}
	ForwardDestination []string `json:"forwardDestination,omitempty"`
	// ForwardKeep states if forwarded e-mail should be kept in the mailbox.
	// +kubebuilder:default=false
	ForwardKeep bool `json:"forwardKeep,omitempty"`
	// GlobalAdmin states if the user has global admin privileges.
	// +kubebuilder:default=false
	GlobalAdmin bool `json:"globalAdmin,omitempty"`
	// PasswordSecret is the name of the secret which contains the password.
	PasswordSecret string `json:"passwordSecret,omitempty"`
	// PasswordKey is the key in the secret that contains the password.
	PasswordKey string `json:"passwordKey,omitempty"`
	// QuotaBytes defines the storage quota, default -1 for unlimited.
	// +kubebuilder:default=-1
	QuotaBytes int64 `json:"quotaBytes,omitempty"`
	// RawPassword is the plaintext password for user creation.
	RawPassword string `json:"rawPassword,omitempty"`
	// ReplyEnabled states if e-mails should be auto-replied to.
	// +kubebuilder:default=false
	ReplyEnabled bool `json:"replyEnabled,omitempty"`
	// ReplySubject is the subject for auto-reply e-mails.
	// +kubebuilder:default=""
	ReplySubject string `json:"replySubject,omitempty"`
	// ReplyBody is the body for auto-reply e-mails.
	// +kubebuilder:default=""
	ReplyBody string `json:"replyBody,omitempty"`
	// ReplyStartDate is the date from which on auto-reply e-mails should be sent.
	// +kubebuilder:validation:Format=date
	// +kubebuilder:default="1900-01-01"
	ReplyStartDate string `json:"replyStartDate,omitempty"`
	// ReplyEndDate is the date until which auto-reply e-mails should be sent.
	// +kubebuilder:validation:Format=date
	// +kubebuilder:default="2999-12-31"
	ReplyEndDate string `json:"replyEndDate,omitempty"`
	// SpamEnabled states if e-mail should be scanned for SPAM.
	// +kubebuilder:default=false
	SpamEnabled bool `json:"spamEnabled,omitempty"`
	// SpamMarkAsRead states if identified SPAM e-mails should be marked as read.
	// +kubebuilder:default=false
	SpamMarkAsRead bool `json:"spamMarkAsRead,omitempty"`
	// SpamThreshold is the threshold for the SPAM filter.
	// +kubebuilder:default=0
	SpamThreshold int `json:"spamThreshold,omitempty"`
}

// UserStatus defines the observed state of User
type UserStatus struct {
	// +operator-sdk:csv:customresourcedefinitions:type=status
	Conditions []metav1.Condition `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type"`
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
