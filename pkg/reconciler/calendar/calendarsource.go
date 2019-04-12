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
	"k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"log"
	"os"

	"github.com/knative/eventing-sources/pkg/controller/sdk"
	"github.com/knative/eventing-sources/pkg/controller/sinks"
	"github.com/knative/pkg/logging"
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
		Owns:      []runtime.Object{&v1.Deployment{}},
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

	_, err := r.secretFrom(ctx, source)
	if err != nil {
		return err
	}
	source.Status.MarkSecrets()

	uri, err := r.sinkURIFrom(ctx, source)
	if err != nil {
		return err
	}
	source.Status.MarkSink(uri)

	_, err = r.reconcileDeployment(ctx, source)
	if err != nil {
		return err
	}
	source.Status.MarkDeployment()

	return nil
}

func (r *reconciler) finalize(ctx context.Context, source *sourcesv1alpha1.CalendarSource) error {
	r.removeFinalizer(source)
	return nil
}

func (r *reconciler) reconcileDeployment(ctx context.Context, source *sourcesv1alpha1.CalendarSource) (*v1.Deployment, error) {
	current, err := r.getDeployment(ctx, source)

	// If the resource doesn't exist, we'll create it.
	if apierrors.IsNotFound(err) {
		d, err := r.newDeployment(source)
		if err != nil {
			return nil, err
		}
		err = r.client.Create(ctx, d)
		if err != nil {
			source.Status.MarkNoDeployment("DeploymentCreateFailed", "%s", err)
			return nil, err
		}
		return d, nil
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

func (r *reconciler) secretFrom(ctx context.Context, source *sourcesv1alpha1.CalendarSource) (string, error) {
	secret := &corev1.Secret{}
	err := r.client.Get(ctx, client.ObjectKey{Namespace: source.Namespace, Name: source.Spec.GcpCredsSecret.Name}, secret)
	if err != nil {
		source.Status.MarkNoSecrets("GcpCredsSecretNotFound", "%s", err)
		return "", err
	}
	secretVal, ok := secret.Data[source.Spec.GcpCredsSecret.Key]
	if !ok {
		return "", fmt.Errorf("key %q not found in secret %q", source.Spec.GcpCredsSecret.Key, source.Spec.GcpCredsSecret.Name)
	}
	return string(secretVal), nil
}

func (r *reconciler) getDeployment(ctx context.Context, src *sourcesv1alpha1.CalendarSource) (*v1.Deployment, error) {
	dl := &v1.DeploymentList{}
	err := r.client.List(ctx, &client.ListOptions{
		Namespace:     src.Namespace,
		LabelSelector: r.getLabelSelector(src),
		// TODO this is only needed by the fake client. Real K8s does not need it. Remove it once
		// the fake is fixed.
		Raw: &metav1.ListOptions{
			TypeMeta: metav1.TypeMeta{
				APIVersion: v1.SchemeGroupVersion.String(),
				Kind:       "Deployment",
			},
		},
	}, dl)
	if err != nil {
		return nil, err
	}
	for _, dep := range dl.Items {
		if metav1.IsControlledBy(&dep, src) {
			return &dep, nil
		}
	}
	return nil, apierrors.NewNotFound(schema.GroupResource{}, "deployments")
}

func (r *reconciler) getLabelSelector(src *sourcesv1alpha1.CalendarSource) labels.Selector {
	return labels.SelectorFromSet(getLabels(src))
}

func getLabels(src *sourcesv1alpha1.CalendarSource) map[string]string {
	return map[string]string{
		"knative-eventing-source":      controllerAgentName,
		"knative-eventing-source-name": src.Name,
	}
}

func (r *reconciler) newDeployment(source *sourcesv1alpha1.CalendarSource) (*v1.Deployment, error) {
	args := &resources.DeploymentArgs{
		Image:   r.receiveAdapterImage,
		Source:  source,
		Labels:  getLabels(source),
		SinkURI: source.Status.SinkURI,
	}
	depl := resources.MakeDeployment(args)
	if err := controllerutil.SetControllerReference(source, depl, r.scheme); err != nil {
		return nil, err
	}
	return depl, nil
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
