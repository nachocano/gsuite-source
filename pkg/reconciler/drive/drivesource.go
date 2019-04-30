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

package drive

import (
	"context"
	"fmt"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/option"
	"io/ioutil"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/uuid"
	"log"
	"os"

	"github.com/knative/eventing-sources/pkg/controller/sdk"
	"github.com/knative/eventing-sources/pkg/controller/sinks"
	"github.com/knative/pkg/logging"
	servingv1alpha1 "github.com/knative/serving/pkg/apis/serving/v1alpha1"
	sourcesv1alpha1 "github.com/nachocano/gsuite-source/pkg/apis/sources/v1alpha1"
	"github.com/nachocano/gsuite-source/pkg/reconciler/drive/resources"
	"go.uber.org/zap"
	gsdrive "google.golang.org/api/drive/v3"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

// TODO create a generic controller to avoid duplicating code.

const (
	// controllerAgentName is the string used by this controller to identify
	// itself when creating events.
	controllerAgentName = "drive-source-controller"
	raImageEnvVar       = "DRIVE_RA_IMAGE"
	finalizerName       = controllerAgentName

	credsMountPath = "/var/secrets/google"
)

type webhookArgs struct {
	id          string
	token       string
	domain      string
	credentials string
	email       string
}

// Add creates a new DriveSource Controller and adds it to the
// Manager with default RBAC. The Manager will set fields on the
// Controller and Start it when the Manager is Started.
func Add(mgr manager.Manager, logger *zap.SugaredLogger) error {
	receiveAdapterImage, defined := os.LookupEnv(raImageEnvVar)
	if !defined {
		return fmt.Errorf("required environment variable %q not defined", raImageEnvVar)
	}

	log.Println("Adding the Drive Source Controller")
	p := &sdk.Provider{
		AgentName: controllerAgentName,
		Parent:    &sourcesv1alpha1.DriveSource{},
		Owns:      []runtime.Object{&servingv1alpha1.Service{}},
		Reconciler: &reconciler{
			recorder:            mgr.GetRecorder(controllerAgentName),
			scheme:              mgr.GetScheme(),
			receiveAdapterImage: receiveAdapterImage,
		},
	}

	return p.Add(mgr, logger)
}

// reconciler reconciles a DriveSource object.
type reconciler struct {
	client              client.Client
	scheme              *runtime.Scheme
	recorder            record.EventRecorder
	receiveAdapterImage string
}

// Reconcile reads that state of the cluster for a DriveSource
// object and makes changes based on the state read and what is in the
// DriveSource.Spec.
func (r *reconciler) Reconcile(ctx context.Context, object runtime.Object) error {
	logger := logging.FromContext(ctx)

	source, ok := object.(*sourcesv1alpha1.DriveSource)
	if !ok {
		logger.Errorf("could not find Drive source %v", object)
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

func (r *reconciler) reconcile(ctx context.Context, source *sourcesv1alpha1.DriveSource) error {
	logger := logging.FromContext(ctx)

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
	logger.Infof("Sink URI %s", uri)

	ksvc, err := r.reconcileService(ctx, source)
	if err != nil {
		return err
	}

	domain, err := r.domainFrom(ksvc, source)
	if err != nil {
		// Returning nil on purpose as we will wait until the next reconciliation process is triggered.
		return nil
	}
	logger.Infof("Service domain %s", domain)
	source.Status.MarkService()

	webhookId, webhookResourceId, err := r.reconcileWebhook(ctx, source, domain)
	if err != nil {
		return err
	}
	source.Status.MarkWebHook(webhookId, webhookResourceId)
	logger.Infof("WebHook Id %s - ResourceId %s", webhookId, webhookResourceId)
	return nil
}

func (r *reconciler) finalize(ctx context.Context, source *sourcesv1alpha1.DriveSource) error {
	logger := logging.FromContext(ctx)
	r.removeFinalizer(source)
	if source.Status.WebhookId != "" && source.Status.WebhookResourceId != "" {
		svc, err := r.createDriveService(ctx, source.Spec.GcpCredsSecret.Key, source.Spec.EmailAddress)
		if err != nil {
			return err
		}
		channel := &gsdrive.Channel{
			Id:         source.Status.WebhookId,
			ResourceId: source.Status.WebhookResourceId,
		}
		err = svc.Channels.Stop(channel).Do()
		if err != nil {
			return err
		}
		logger.Infof("Successfully removed Webhook Id %s - ResourceId %s", source.Status.WebhookId, source.Status.WebhookResourceId)
	}
	return nil
}

func (r *reconciler) domainFrom(ksvc *servingv1alpha1.Service, source *sourcesv1alpha1.DriveSource) (string, error) {
	routeCondition := ksvc.Status.GetCondition(servingv1alpha1.ServiceConditionRoutesReady)
	receiveAdapterDomain := ksvc.Status.Domain
	if routeCondition != nil && routeCondition.Status == corev1.ConditionTrue && receiveAdapterDomain != "" {
		return receiveAdapterDomain, nil
	}
	err := fmt.Errorf("domain not found for svc %q", ksvc.Name)
	source.Status.MarkNoService("ServiceDomainNotFound", "%s", err)
	return "", err
}

func (r *reconciler) reconcileService(ctx context.Context, source *sourcesv1alpha1.DriveSource) (*servingv1alpha1.Service, error) {
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

func (r *reconciler) reconcileWebhook(ctx context.Context, source *sourcesv1alpha1.DriveSource, domain string) (string, string, error) {
	// If webhook doesn't exist, then create it.
	if source.Status.WebhookId == "" || source.Status.WebhookResourceId == "" {
		r.addFinalizer(source)

		webhookArgs := &webhookArgs{
			id:          string(uuid.NewUUID()),
			token:       sourcesv1alpha1.DriveSourceToken,
			domain:      domain,
			credentials: source.Spec.GcpCredsSecret.Key,
			email:       source.Spec.EmailAddress,
		}

		id, resourceId, err := r.createWebhook(ctx, webhookArgs)
		if err != nil {
			source.Status.MarkNoWebHook("WebHookCreateFailed", "%s", err)
			return "", "", err
		}
		source.Status.WebhookId = id
		source.Status.WebhookResourceId = resourceId
	}
	return source.Status.WebhookId, source.Status.WebhookResourceId, nil
}

func (r *reconciler) createWebhook(ctx context.Context, args *webhookArgs) (string, string, error) {
	svc, err := r.createDriveService(ctx, args.credentials, args.email)
	if err != nil {
		return "", "", err
	}

	startPageTokenResp, err := svc.Changes.GetStartPageToken().Do()
	if err != nil {
		return "", "", err
	}
	pageToken := startPageTokenResp.StartPageToken
	logging.FromContext(ctx).Infof("StartPageToken %q", pageToken)

	channel := &gsdrive.Channel{
		Id:      args.id,
		Token:   args.token,
		Address: fmt.Sprintf("https://%s", args.domain),
		Kind:    "api#channel",
		Type:    "web_hook",
	}
	resp, err := svc.Changes.Watch(pageToken, channel).Do()
	if err != nil {
		return "", "", err
	}
	// TODO read expiration and trigger some Event to recreate the webhook
	return resp.Id, resp.ResourceId, nil
}

func (r *reconciler) createDriveService(ctx context.Context, credentials, email string) (*gsdrive.Service, error) {
	// Doing this as there is no way to impersonate a particular user drive
	// using the GOOGLE_APPLICATION_CREDENTIALS env variable.
	credsFile := fmt.Sprintf("%s/%s", credsMountPath, credentials)
	jsonCredentials, err := ioutil.ReadFile(credsFile)
	if err != nil {
		return nil, err
	}
	conf, err := google.JWTConfigFromJSON(jsonCredentials, gsdrive.DriveReadonlyScope)
	if err != nil {
		return nil, err
	}
	// Impersonate the following user using the service account credentials
	conf.Subject = email

	client := conf.Client(ctx)
	return gsdrive.NewService(ctx, option.WithHTTPClient(client))
}

func (r *reconciler) sinkURIFrom(ctx context.Context, source *sourcesv1alpha1.DriveSource) (string, error) {
	uri, err := sinks.GetSinkURI(ctx, r.client, source.Spec.Sink, source.Namespace)
	if err != nil {
		source.Status.MarkNoSink("SinkNotFound", "%s", err)
		return "", err
	}
	return uri, err
}

func (r *reconciler) secretFrom(ctx context.Context, source *sourcesv1alpha1.DriveSource) (string, error) {
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

func (r *reconciler) getService(ctx context.Context, source *sourcesv1alpha1.DriveSource) (*servingv1alpha1.Service, error) {
	list := &servingv1alpha1.ServiceList{}
	err := r.client.List(ctx, &client.ListOptions{
		Namespace:     source.Namespace,
		LabelSelector: labels.Everything(),
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

func (r *reconciler) newService(source *sourcesv1alpha1.DriveSource) (*servingv1alpha1.Service, error) {
	ksvc := resources.MakeService(source, r.receiveAdapterImage)
	if err := controllerutil.SetControllerReference(source, ksvc, r.scheme); err != nil {
		return nil, err
	}
	return ksvc, nil
}

func (r *reconciler) addFinalizer(s *sourcesv1alpha1.DriveSource) {
	finalizers := sets.NewString(s.Finalizers...)
	finalizers.Insert(finalizerName)
	s.Finalizers = finalizers.List()
}

func (r *reconciler) removeFinalizer(s *sourcesv1alpha1.DriveSource) {
	finalizers := sets.NewString(s.Finalizers...)
	finalizers.Delete(finalizerName)
	s.Finalizers = finalizers.List()
}

func (r *reconciler) InjectClient(c client.Client) error {
	r.client = c
	return nil
}
