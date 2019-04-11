/*
Copyright 2019 The Knative Authors

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
	"github.com/knative/pkg/apis/duck"
	duckv1alpha1 "github.com/knative/pkg/apis/duck/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

// Check that SheetsSource can be validated and can be defaulted.
var _ runtime.Object = (*SheetsSource)(nil)

// Check that SheetsSource implements the Conditions duck type.
var _ = duck.VerifyType(&SheetsSource{}, &duckv1alpha1.Conditions{})

// SheetsSourceSpec defines the desired state of SheetsSourceSpec.
type SheetsSourceSpec struct {
	// +optional
	ServiceAccountName string `json:"serviceAccountName,omitempty"`

	// +kubebuilder:validation:MinLength=1
	OwnerAndRepository string `json:"ownerAndRepository"`

	// +kubebuilder:validation:MinItems=1
	EventTypes []string `json:"eventTypes"`

	AccessToken SecretFromSource `json:"accessToken"`

	SecretToken SecretFromSource `json:"secretToken"`

	// +optional
	Sink *corev1.ObjectReference `json:"sink,omitempty"`
}

type SecretFromSource struct {
	// The Secret key to select from.
	SecretKeyRef *corev1.SecretKeySelector `json:"secretKeyRef,omitempty"`
}

const (
	// SheetsSourceEventPrefix is what all Sheets event types get
	// prefixed with when converting to CloudEvent EventType
	SheetsSourceEventPrefix = "dev.knative.source.sheets"
)

const (
	// SheetsSourceConditionReady has status True when the
	// SheetsSource is ready to send events.
	SheetsSourceConditionReady = duckv1alpha1.ConditionReady

	// SheetsSourceConditionSecretsProvided has status True when the
	// SheetsSource has valid secret references.
	SheetsSourceConditionSecretsProvided duckv1alpha1.ConditionType = "SecretsProvided"

	// SheetsSourceConditionSinkProvided has status True when the
	// SheetsSource has been configured with a sink target.
	SheetsSourceConditionSinkProvided duckv1alpha1.ConditionType = "SinkProvided"

	// SheetsSourceConditionServiceProvided has status True when the
	// SheetsSource has valid service references.
	SheetsSourceConditionServiceProvided duckv1alpha1.ConditionType = "ServiceProvided"
)

var sheetsSourceCondSet = duckv1alpha1.NewLivingConditionSet(
	SheetsSourceConditionSecretsProvided,
	SheetsSourceConditionSinkProvided,
	SheetsSourceConditionServiceProvided,
)

// SheetsSourceStatus defines the observed state of SheetsSource.
type SheetsSourceStatus struct {
	// inherits duck/v1alpha1 Status, which currently provides:
	// * ObservedGeneration - the 'Generation' of the Service that was last processed by the controller.
	// * Conditions - the latest available observations of a resource's current state.
	duckv1alpha1.Status `json:",inline"`

	// SinkURI is the current active sink URI that has been configured
	// for the SheetsSource.
	// +optional
	SinkURI string `json:"sinkUri,omitempty"`
}

// GetCondition returns the condition currently associated with the given type, or nil.
func (s *SheetsSourceStatus) GetCondition(t duckv1alpha1.ConditionType) *duckv1alpha1.Condition {
	return sheetsSourceCondSet.Manage(s).GetCondition(t)
}

// IsReady returns true if the resource is ready overall.
func (s *SheetsSourceStatus) IsReady() bool {
	return sheetsSourceCondSet.Manage(s).IsHappy()
}

// InitializeConditions sets relevant unset conditions to Unknown state.
func (s *SheetsSourceStatus) InitializeConditions() {
	sheetsSourceCondSet.Manage(s).InitializeConditions()
}

// MarkService sets the condition that the source has a service configured.
func (s *SheetsSourceStatus) MarkService() {
	sheetsSourceCondSet.Manage(s).MarkTrue(SheetsSourceConditionServiceProvided)
}

// MarkNoService sets the condition that the source does not have a valid service.
func (s *SheetsSourceStatus) MarkNoService(reason, messageFormat string, messageA ...interface{}) {
	sheetsSourceCondSet.Manage(s).MarkFalse(SheetsSourceConditionServiceProvided, reason, messageFormat, messageA...)
}

// MarkSecrets sets the condition that the source has a valid secret.
func (s *SheetsSourceStatus) MarkSecrets() {
	sheetsSourceCondSet.Manage(s).MarkTrue(SheetsSourceConditionSecretsProvided)
}

// MarkNoSecrets sets the condition that the source does not have a valid secret.
func (s *SheetsSourceStatus) MarkNoSecrets(reason, messageFormat string, messageA ...interface{}) {
	sheetsSourceCondSet.Manage(s).MarkFalse(SheetsSourceConditionSecretsProvided, reason, messageFormat, messageA...)
}

// MarkSink sets the condition that the source has a sink configured.
func (s *SheetsSourceStatus) MarkSink(uri string) {
	s.SinkURI = uri
	if len(uri) > 0 {
		sheetsSourceCondSet.Manage(s).MarkTrue(SheetsSourceConditionSinkProvided)
	} else {
		sheetsSourceCondSet.Manage(s).MarkUnknown(SheetsSourceConditionSinkProvided,
			"SinkEmpty", "Sink has resolved to empty.")
	}
}

// MarkNoSink sets the condition that the source does not have a sink configured.
func (s *SheetsSourceStatus) MarkNoSink(reason, messageFormat string, messageA ...interface{}) {
	sheetsSourceCondSet.Manage(s).MarkFalse(SheetsSourceConditionSinkProvided, reason, messageFormat, messageA...)
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// SheetsSource is the Schema for the sheetssources API.
// +k8s:openapi-gen=true
// +kubebuilder:subresource:status
// +kubebuilder:categories=all,knative,eventing,sources
type SheetsSource struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   SheetsSourceSpec   `json:"spec,omitempty"`
	Status SheetsSourceStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// SheetsSourceList contains a list of SheetsSource.
type SheetsSourceList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []SheetsSource `json:"items"`
}
