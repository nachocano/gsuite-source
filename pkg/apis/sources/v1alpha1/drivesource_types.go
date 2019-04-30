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

var _ runtime.Object = (*DriveSource)(nil)

var _ = duck.VerifyType(&DriveSource{}, &duckv1alpha1.Conditions{})

type DriveSourceSpec struct {
	// TODO be able to set many, so that we create a single Service listening to events from many drives.
	EmailAddress   string                   `json:"emailAddress"`
	GcpCredsSecret corev1.SecretKeySelector `json:"gcpCredsSecret"`
	Sink           *corev1.ObjectReference  `json:"sink"`
}

const (
	DriveSourceEventType = "org.nachocano.source.gsuite.drive"
	DriveSourceToken     = DriveSourceEventType
)

const (
	DriveSourceConditionReady                                      = duckv1alpha1.ConditionReady
	DriveSourceConditionSecretsProvided duckv1alpha1.ConditionType = "SecretsProvided"
	DriveSourceConditionSinkProvided    duckv1alpha1.ConditionType = "SinkProvided"
	DriveSourceConditionServiceProvided duckv1alpha1.ConditionType = "ServiceProvided"
	DriveSourceConditionWebHookProvided duckv1alpha1.ConditionType = "WebHookProvided"
)

var driveSourceCondSet = duckv1alpha1.NewLivingConditionSet(
	DriveSourceConditionSecretsProvided,
	DriveSourceConditionSinkProvided,
	DriveSourceConditionServiceProvided,
	DriveSourceConditionWebHookProvided,
)

type DriveSourceStatus struct {
	duckv1alpha1.Status `json:",inline"`

	WebhookId         string `json:"webhookId,omitempty"`
	WebhookResourceId string `json:"webhookResourceId,omitempty"`

	SinkURI string `json:"sinkUri,omitempty"`
}

// GetCondition returns the condition currently associated with the given type, or nil.
func (s *DriveSourceStatus) GetCondition(t duckv1alpha1.ConditionType) *duckv1alpha1.Condition {
	return driveSourceCondSet.Manage(s).GetCondition(t)
}

// IsReady returns true if the resource is ready overall.
func (s *DriveSourceStatus) IsReady() bool {
	return driveSourceCondSet.Manage(s).IsHappy()
}

// InitializeConditions sets relevant unset conditions to Unknown state.
func (s *DriveSourceStatus) InitializeConditions() {
	driveSourceCondSet.Manage(s).InitializeConditions()
}

// MarkService sets the condition that the source has a service configured.
func (s *DriveSourceStatus) MarkService() {
	driveSourceCondSet.Manage(s).MarkTrue(DriveSourceConditionServiceProvided)
}

// MarkNoService sets the condition that the source does not have a valid service.
func (s *DriveSourceStatus) MarkNoService(reason, messageFormat string, messageA ...interface{}) {
	driveSourceCondSet.Manage(s).MarkFalse(DriveSourceConditionServiceProvided, reason, messageFormat, messageA...)
}

// MarkWebHook sets the condition that the source has a webhook configured.
func (s *DriveSourceStatus) MarkWebHook(id, resourceId string) {
	s.WebhookId = id
	s.WebhookResourceId = resourceId
	if len(id) > 0 && len(resourceId) > 0 {
		driveSourceCondSet.Manage(s).MarkTrue(DriveSourceConditionWebHookProvided)
	} else {
		driveSourceCondSet.Manage(s).MarkFalse(DriveSourceConditionWebHookProvided,
			"WebHookParamsEmpty", "WebHookParams empty.")
	}

}

// MarkNoWebHook sets the condition that the source does not have a valid webhook.
func (s *DriveSourceStatus) MarkNoWebHook(reason, messageFormat string, messageA ...interface{}) {
	s.WebhookId = ""
	s.WebhookResourceId = ""
	driveSourceCondSet.Manage(s).MarkFalse(DriveSourceConditionWebHookProvided, reason, messageFormat, messageA...)
}

// MarkSecrets sets the condition that the source has a valid secret.
func (s *DriveSourceStatus) MarkSecrets() {
	driveSourceCondSet.Manage(s).MarkTrue(DriveSourceConditionSecretsProvided)
}

// MarkNoSecrets sets the condition that the source does not have a valid secret.
func (s *DriveSourceStatus) MarkNoSecrets(reason, messageFormat string, messageA ...interface{}) {
	driveSourceCondSet.Manage(s).MarkFalse(DriveSourceConditionSecretsProvided, reason, messageFormat, messageA...)
}

// MarkSink sets the condition that the source has a sink configured.
func (s *DriveSourceStatus) MarkSink(uri string) {
	s.SinkURI = uri
	if len(uri) > 0 {
		driveSourceCondSet.Manage(s).MarkTrue(DriveSourceConditionSinkProvided)
	} else {
		driveSourceCondSet.Manage(s).MarkUnknown(DriveSourceConditionSinkProvided,
			"SinkEmpty", "Sink has resolved to empty.")
	}
}

// MarkNoSink sets the condition that the source does not have a sink configured.
func (s *DriveSourceStatus) MarkNoSink(reason, messageFormat string, messageA ...interface{}) {
	driveSourceCondSet.Manage(s).MarkFalse(DriveSourceConditionSinkProvided, reason, messageFormat, messageA...)
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// DriveSource is the Schema for the drivesources API.
// +k8s:openapi-gen=true
// +kubebuilder:subresource:status
// +kubebuilder:categories=all,knative,eventing,sources
type DriveSource struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   DriveSourceSpec   `json:"spec,omitempty"`
	Status DriveSourceStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// DriveSourceList contains a list of DriveSource.
type DriveSourceList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []DriveSource `json:"items"`
}
