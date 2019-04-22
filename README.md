# Knative G Suite Sources

This repository implements Event Sources to wire G Suite events into [Knative Eventing](https://github.com/knative/eventing).

## Prerequisites

You will need:

1. A [Google Cloud project](https://cloud.google.com/resource-manager/docs/creating-managing-projects) and install the 
[gcloud CLI](https://cloud.google.com/sdk/) and run `gcloud auth login`. 
The samples below will use a mix of `gcloud` and `kubectl` commands. 
We assume that you have set the `$PROJECT_ID` environment variable to your GCP 
project ID, and that you set your project ID as default using 
`gcloud config set project $PROJECT_ID`.
1. An internet-accessible Kubernetes cluster with Knative Serving
   installed. Follow the [installation
   instructions](https://www.knative.dev/docs/install/)
   if you need to create one.
1. Ensure Knative Serving is [configured with a domain
   name](https://www.knative.dev/docs/serving/using-a-custom-domain/)
   that allows G Suite to call into the cluster.
1. Ensure Knative Serving is [configured with HTTPS with a custom 
certificate](https://knative.dev/docs/serving/using-an-ssl-cert/) as 
G Suite Push Notifications require HTTPS and valid certificates.
1. If you're using GKE, you'll also want to [assign a static IP address](https://www.knative.dev/docs/serving/gke-assigning-static-ip-address/).
1. Install [Knative Eventing](https://www.knative.dev/docs/install/index.html). Those
   instructions also install the default eventing sources.
1. A G Suite domain where you have administrator privileges, as we 
will do [G Suite Domain-Wide Delegation of Authority](https://developers.google.com/identity/protocols/OAuth2ServiceAccount#delegatingauthority).   


## GCP Service Account and Kubernetes Secrets

1. Create a [GCP Service Account](https://console.cloud.google.com/iam-admin/serviceaccounts/project). 
All the examples below use the same service account but you can create different ones for the different G Suite applications.
 Create a new service account named gcs-source with the following command:

    1. Create a new service account named `gsuite-source` with the following
       command: 
       ```shell
       gcloud iam service-accounts create gsuite-source
       ```
    1. Give that service account the viewer role for your GCP project:
       ```shell 
       gcloud projects add-iam-policy-binding $PROJECT_ID \
         --member=serviceAccount:gsuite-source@$PROJECT_ID.iam.gserviceaccount.com \
         --role roles/viewer
       ```
    1. Edit that service account from the [GCP UI](https://console.cloud.google.com/iam-admin/serviceaccounts/project), 
       and mark the checkbox to Enable G Suite domain-wide delegation. 
       Also, copy the Client ID to some notes as you will use it later. 
    1. Download a new JSON private key for that service account.
       ```shell
        gcloud iam service-accounts keys create gsuite-source.json \
          --iam-account=gsuite-source@$PROJECT_ID.iam.gserviceaccount.com
       ```
    1. Create a namespace where the secret is created and where our controller will run
       ```shell
       kubectl create namespace gsuite-sources
       ```
    1. Create a secret on the Kubernetes cluster for the downloaded key. You need
      to store this key in `key.json` in a secret named `gsuite-source-key`. 
      This is used by the `controller` to create web hooks to G Suite Push notifications.
      ```shell 
      kubectl -n gsuite-sources create secret generic gsuite-source-key \
        --from-file=key.json=gsuite-source.json --dry-run -o yaml | kubectl apply --filename -
      ```

## Install G Suite Sources

Install the G Suite sources by executing:
    
```shell
`ko apply -f ./config`
```

Wait until the controller has `Running` status:

```shell
kubectl get pods -n gsuite-sources 
```

The G Suite controller is up and running! 

## G Suite Sources CRDs

Below you can find the list of the currently supported G Suite sources CRDs that are packaged with 
this installation and their respective examples.

| Name | Status | Support | Description |
|------|--------|---------|-------------|
| [Calendar](./samples/calendar/README.md) | Proof of Concept | None | Brings [Google Calendar](https://calendar.google.com/calendar/) events into Knative |


#### Cleanup

You can remove the G Suite sources by deleting the namespace:

```shell
kubectl delete namespace gsuite-sources
```


 