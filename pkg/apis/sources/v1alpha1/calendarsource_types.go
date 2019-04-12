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

var _ runtime.Object = (*CalendarSource)(nil)

var _ = duck.VerifyType(&CalendarSource{}, &duckv1alpha1.Conditions{})

type CalendarSourceSpec struct {
	// +optional
	ServiceAccountName string `json:"serviceAccountName,omitempty"`

	GcpCredsSecret corev1.SecretKeySelector `json:"gcpCredsSecret"`

	// +optional
	Sink *corev1.ObjectReference `json:"sink,omitempty"`
}

type CalendarSecretFromSource struct {
	// The Secret key to select from.
	SecretKeyRef *corev1.SecretKeySelector `json:"secretKeyRef,omitempty"`
}

const (
	CalendarSourceEventPrefix = "dev.knative.source.calendar"
)

const (
	CalendarSourceConditionReady                                         = duckv1alpha1.ConditionReady
	CalendarSourceConditionSecretsProvided    duckv1alpha1.ConditionType = "SecretsProvided"
	CalendarSourceConditionSinkProvided       duckv1alpha1.ConditionType = "SinkProvided"
	CalendarSourceConditionDeploymentProvided duckv1alpha1.ConditionType = "DeploymentProvided"
)

var calendarSourceCondSet = duckv1alpha1.NewLivingConditionSet(
	CalendarSourceConditionSecretsProvided,
	CalendarSourceConditionSinkProvided,
	CalendarSourceConditionDeploymentProvided,
)

type CalendarSourceStatus struct {
	duckv1alpha1.Status `json:",inline"`

	SinkURI string `json:"sinkUri,omitempty"`
}

// GetCondition returns the condition currently associated with the given type, or nil.
func (s *CalendarSourceStatus) GetCondition(t duckv1alpha1.ConditionType) *duckv1alpha1.Condition {
	return calendarSourceCondSet.Manage(s).GetCondition(t)
}

// IsReady returns true if the resource is ready overall.
func (s *CalendarSourceStatus) IsReady() bool {
	return calendarSourceCondSet.Manage(s).IsHappy()
}

// InitializeConditions sets relevant unset conditions to Unknown state.
func (s *CalendarSourceStatus) InitializeConditions() {
	calendarSourceCondSet.Manage(s).InitializeConditions()
}

// MarkDeployment sets the condition that the source has a deployment configured.
func (s *CalendarSourceStatus) MarkDeployment() {
	calendarSourceCondSet.Manage(s).MarkTrue(CalendarSourceConditionDeploymentProvided)
}

// MarkNoDeployment sets the condition that the source does not have a valid deployment.
func (s *CalendarSourceStatus) MarkNoDeployment(reason, messageFormat string, messageA ...interface{}) {
	calendarSourceCondSet.Manage(s).MarkFalse(CalendarSourceConditionDeploymentProvided, reason, messageFormat, messageA...)
}

// MarkSecrets sets the condition that the source has a valid secret.
func (s *CalendarSourceStatus) MarkSecrets() {
	calendarSourceCondSet.Manage(s).MarkTrue(CalendarSourceConditionSecretsProvided)
}

// MarkNoSecrets sets the condition that the source does not have a valid secret.
func (s *CalendarSourceStatus) MarkNoSecrets(reason, messageFormat string, messageA ...interface{}) {
	calendarSourceCondSet.Manage(s).MarkFalse(CalendarSourceConditionSecretsProvided, reason, messageFormat, messageA...)
}

// MarkSink sets the condition that the source has a sink configured.
func (s *CalendarSourceStatus) MarkSink(uri string) {
	s.SinkURI = uri
	if len(uri) > 0 {
		calendarSourceCondSet.Manage(s).MarkTrue(CalendarSourceConditionSinkProvided)
	} else {
		calendarSourceCondSet.Manage(s).MarkUnknown(CalendarSourceConditionSinkProvided,
			"SinkEmpty", "Sink has resolved to empty.")
	}
}

// MarkNoSink sets the condition that the source does not have a sink configured.
func (s *CalendarSourceStatus) MarkNoSink(reason, messageFormat string, messageA ...interface{}) {
	calendarSourceCondSet.Manage(s).MarkFalse(CalendarSourceConditionSinkProvided, reason, messageFormat, messageA...)
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// CalendarSource is the Schema for the calendarsources API.
// +k8s:openapi-gen=true
// +kubebuilder:subresource:status
// +kubebuilder:categories=all,knative,eventing,sources
type CalendarSource struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   CalendarSourceSpec   `json:"spec,omitempty"`
	Status CalendarSourceStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// CalendarSourceList contains a list of CalendarSource.
type CalendarSourceList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []CalendarSource `json:"items"`
}