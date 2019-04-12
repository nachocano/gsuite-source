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

package calendar

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/knative/eventing-sources/pkg/controller/sdk"
	"github.com/knative/eventing-sources/pkg/controller/sinks"
	"github.com/knative/pkg/logging"
	servingv1alpha1 "github.com/knative/serving/pkg/apis/serving/v1alpha1"
	sourcesv1alpha1 "github.com/nachocano/gsuite-source/pkg/apis/sources/v1alpha1"
	"github.com/nachocano/gsuite-source/pkg/reconciler/calendar/resources"
	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

const (
	// controllerAgentName is the string used by this controller to identify
	// itself when creating events.
	controllerAgentName = "calendar-source-controller"
	raImageEnvVar       = "CALENDAR_RA_IMAGE"
	finalizerName       = controllerAgentName
)

// Add creates a new CalendarSource Controller and adds it to the
// Manager with default RBAC. The Manager will set fields on the
// Controller and Start it when the Manager is Started.
func Add(mgr manager.Manager) error {
	receiveAdapterImage, defined := os.LookupEnv(raImageEnvVar)
	if !defined {
		return fmt.Errorf("required environment variable %q not defined", raImageEnvVar)
	}

	log.Println("Adding the Calendar Source Controller")
	p := &sdk.Provider{
		AgentName: controllerAgentName,
		Parent:    &sourcesv1alpha1.CalendarSource{},
		Owns:      []runtime.Object{&servingv1alpha1.Service{}},
		Reconciler: &reconciler{
			recorder:            mgr.GetRecorder(controllerAgentName),
			scheme:              mgr.GetScheme(),
			receiveAdapterImage: receiveAdapterImage,
		},
	}

	return p.Add(mgr)
}

// reconciler reconciles a CalendarSource object.
type reconciler struct {
	client              client.Client
	scheme              *runtime.Scheme
	recorder            record.EventRecorder
	receiveAdapterImage string
}

// Reconcile reads that state of the cluster for a CalendarSource
// object and makes changes based on the state read and what is in the
// CalendarSource.Spec.
func (r *reconciler) Reconcile(ctx context.Context, object runtime.Object) error {
	logger := logging.FromContext(ctx)

	source, ok := object.(*sourcesv1alpha1.CalendarSource)
	if !ok {
		logger.Errorf("could not find Calendar source %v", object)
		return nil
	}

	// See if the source has been deleted.
	accessor, err := meta.Accessor(source)
	if err != nil {
		logger.Warnf("Failed to get metadata accessor: %s", zap.Error(err))
		return err
	}

	var reconcileErr error
	if accessor.GetDeletionTimestamp() == nil {
		reconcileErr = r.reconcile(ctx, source)
	} else {
		reconcileErr = r.finalize(ctx, source)
	}

	return reconcileErr
}

func (r *reconciler) reconcile(ctx context.Context, source *sourcesv1alpha1.CalendarSource) error {
	source.Status.InitializeConditions()

	_, _, err := r.secretsFrom(ctx, source)
	if err != nil {
		return err
	}
	source.Status.MarkSecrets()

	uri, err := r.sinkURIFrom(ctx, source)
	if err != nil {
		return err
	}
	source.Status.MarkSink(uri)

	ksvc, err := r.reconcileService(ctx, source)
	if err != nil {
		return err
	}

	_, err = r.domainFrom(ksvc, source)
	if err != nil {
		// Returning nil on purpose as we will wait until the next reconciliation process is triggered.
		return nil
	}
	source.Status.MarkService()

	return nil
}

func (r *reconciler) finalize(ctx context.Context, source *sourcesv1alpha1.CalendarSource) error {
	r.removeFinalizer(source)
	return nil
}

func (r *reconciler) domainFrom(ksvc *servingv1alpha1.Service, source *sourcesv1alpha1.CalendarSource) (string, error) {
	routeCondition := ksvc.Status.GetCondition(servingv1alpha1.ServiceConditionRoutesReady)
	receiveAdapterDomain := ksvc.Status.Domain
	if routeCondition != nil && routeCondition.Status == corev1.ConditionTrue && receiveAdapterDomain != "" {
		return receiveAdapterDomain, nil
	}
	err := fmt.Errorf("domain not found for svc %q", ksvc.Name)
	source.Status.MarkNoService("ServiceDomainNotFound", "%s", err)
	return "", err
}

func (r *reconciler) reconcileService(ctx context.Context, source *sourcesv1alpha1.CalendarSource) (*servingv1alpha1.Service, error) {
	current, err := r.getService(ctx, source)

	// If the resource doesn't exist, we'll create it.
	if apierrors.IsNotFound(err) {
		ksvc, err := r.newService(source)
		if err != nil {
			return nil, err
		}
		err = r.client.Create(ctx, ksvc)
		if err != nil {
			source.Status.MarkNoService("ServiceCreateFailed", "%s", err)
			return nil, err
		}
		return ksvc, nil
	} else if err != nil {
		return nil, err
	}

	return current, nil
}

func (r *reconciler) sinkURIFrom(ctx context.Context, source *sourcesv1alpha1.CalendarSource) (string, error) {
	uri, err := sinks.GetSinkURI(ctx, r.client, source.Spec.Sink, source.Namespace)
	if err != nil {
		source.Status.MarkNoSink("SinkNotFound", "%s", err)
		return "", err
	}
	return uri, err
}

func (r *reconciler) secretsFrom(ctx context.Context, source *sourcesv1alpha1.CalendarSource) (string, string, error) {

	accessToken, err := r.secretFrom(ctx, source.Namespace, source.Spec.AccessToken.SecretKeyRef)
	if err != nil {
		source.Status.MarkNoSecrets("AccessTokenNotFound", "%s", err)
		return "", "", err
	}
	secretToken, err := r.secretFrom(ctx, source.Namespace, source.Spec.SecretToken.SecretKeyRef)
	if err != nil {
		source.Status.MarkNoSecrets("SecretTokenNotFound", "%s", err)
		return "", "", err
	}

	return accessToken, secretToken, err
}

func (r *reconciler) secretFrom(ctx context.Context, namespace string, secretKeySelector *corev1.SecretKeySelector) (string, error) {
	if secretKeySelector == nil {
		return "", fmt.Errorf("nil secret key selector")
	}

	secret := &corev1.Secret{}
	err := r.client.Get(ctx, client.ObjectKey{Namespace: namespace, Name: secretKeySelector.Name}, secret)
	if err != nil {
		return "", err
	}
	secretVal, ok := secret.Data[secretKeySelector.Key]
	if !ok {
		return "", fmt.Errorf("key %q not found in secret %q", secretKeySelector.Key, secretKeySelector.Name)
	}
	return string(secretVal), nil
}

func (r *reconciler) ownerRepoFrom(source *sourcesv1alpha1.CalendarSource) (string, string, error) {
	ownerAndRepository := source.Spec.OwnerAndRepository
	components := strings.Split(ownerAndRepository, "/")
	if len(components) > 2 {
		err := fmt.Errorf("ownerAndRepository is malformatted, expected 'owner/repository' but found %q", ownerAndRepository)
		return "", "", err
	}
	owner := components[0]
	if len(owner) == 0 && len(components) > 1 {
		err := fmt.Errorf("owner is empty, expected 'owner/repository' but found %q", ownerAndRepository)
		return "", "", err
	}
	repo := ""
	if len(components) > 1 {
		repo = components[1]
	}

	return owner, repo, nil
}

func (r *reconciler) getService(ctx context.Context, source *sourcesv1alpha1.CalendarSource) (*servingv1alpha1.Service, error) {
	list := &servingv1alpha1.ServiceList{}
	err := r.client.List(ctx, &client.ListOptions{
		Namespace:     source.Namespace,
		LabelSelector: labels.Everything(),
		// TODO this is here because the fake client needs it.
		// Remove this when it's no longer needed.
		Raw: &metav1.ListOptions{
			TypeMeta: metav1.TypeMeta{
				APIVersion: servingv1alpha1.SchemeGroupVersion.String(),
				Kind:       "Service",
			},
		},
	},
		list)
	if err != nil {
		return nil, err
	}
	for _, ksvc := range list.Items {
		if metav1.IsControlledBy(&ksvc, source) {
			return &ksvc, nil
		}
	}
	return nil, apierrors.NewNotFound(servingv1alpha1.Resource("services"), "")
}

func (r *reconciler) newService(source *sourcesv1alpha1.CalendarSource) (*servingv1alpha1.Service, error) {
	ksvc := resources.MakeService(source, r.receiveAdapterImage)
	if err := controllerutil.SetControllerReference(source, ksvc, r.scheme); err != nil {
		return nil, err
	}
	return ksvc, nil
}

func (r *reconciler) addFinalizer(s *sourcesv1alpha1.CalendarSource) {
	finalizers := sets.NewString(s.Finalizers...)
	finalizers.Insert(finalizerName)
	s.Finalizers = finalizers.List()
}

func (r *reconciler) removeFinalizer(s *sourcesv1alpha1.CalendarSource) {
	finalizers := sets.NewString(s.Finalizers...)
	finalizers.Delete(finalizerName)
	s.Finalizers = finalizers.List()
}

func (r *reconciler) InjectClient(c client.Client) error {
	r.client = c
	return nil
}
