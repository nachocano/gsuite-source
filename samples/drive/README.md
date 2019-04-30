# Google Drive Source 

This sample shows how to wire Google Drive events into Knative Eventing.

## Prerequisites

You will need:

1. Follow these [prerequisites](https://github.com/nachocano/gsuite-source#prerequisites).
1. Enable Google Drive API in your GCP project by executing the following command: 
    ```shell
    gcloud services enable drive.googleapis.com
    ```
1. Register your domain to be able to receive push notifications. Follow [these](https://developers.google.com/drive/api/v3/push#registering-your-domain) steps.
1. Delegate domain-wide authority to your service account. 
Follow [these](https://developers.google.com/drive/api/v3/about-auth#perform_g_suite_domain-wide_delegation_of_authority) steps, and
    1. When specifying the API scopes, enter the drive read-only scope: `https://www.googleapis.com/auth/drive.readonly`. 
    1. When asked for the Client ID, enter the your service account's one that you saved during the previous prerequisites.

## Details
The actual implementation contacts the Google Drive API in order to create a 
channel, which is basically a webhook, to receive Push Notifications on Drive event changes. 
The authentication is delegated to the service account, thus no user involvement is required.    
The notifications are delivered to a Knative Service (listening on an HTTPS public address), which converts  
the messages into [CloudEvents](https://github.com/cloudevents/spec) and forwards them to the configured sink.

## Drive Source Spec Fields

The `DriveSource` watches for [Drive changes](https://developers.google.com/drive/api/v3/reference/changes/watch). 
Here are its `spec` fields:

- `emailAddress`: `string` The user email address corresponding to the drive events we are interested in. Must be set.

- `gcpCredsSecret`: a [SecretKeySelector](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.12/#secretkeyselector-v1-core)
  containing the service account secret that will take care of the authentication. Must be set.
- `sink`:
  [ObjectReference](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.12/#objectreference-v1-core)
  A reference to the object that should receive events. Must be set.

## Example

Now we are going to show an example of how to consume Drive events.

### Create a Knative Service

To verify the `DriveSource` is working, we will create a simple Knative Service that dumps incoming messages to its log. 
The `service.yaml` file defines this basic service.

```yaml
apiVersion: serving.knative.dev/v1alpha1
kind: Service
metadata:
  name: drive-event-display
spec:
  runLatest:
    configuration:
      revisionTemplate:
        spec:
          container:
            image: gcr.io/knative-releases/github.com/knative/eventing-sources/cmd/event_display@sha256:bf45b3eb1e7fc4cb63d6a5a6416cf696295484a7662e0cf9ccdf5c080542c21d
```

Enter the following command to create the service from `service.yaml`:

```shell
kubectl -n default apply -f service.yaml
```

### Create an Event Source for Drive Events

In order to receive Drive events, you have to create a concrete 
`DriveSource` CO in a specific namespace. Be sure to replace the
`emailAddress` value with a valid email address in your G Suite domain.

```yaml
apiVersion: sources.nachocano.org/v1alpha1
kind: DriveSource
metadata:
  name: drive-source-sample
spec:
  emailAddress: <YOUR EMAIL ADDRESS>
  gcpCredsSecret:
    name: gs-source-key
    key: key.json
  sink:
    apiVersion: serving.knative.dev/v1alpha1
    kind: Service
    name: drive-event-display
```

Then, apply that yaml using `kubectl`:

```shell
kubectl -n default apply -f drive-source.yaml
```

### Verify

Verify that the `DriveSource` is ready by executing the following command:

```shell
kubectl get drivesources
```
```
NAME                  READY   REASON
drive-source-sample   True
```

### Create Events

Create a drive event in the user's email address Drive. 
We will verify that the Drive event was sent to the Knative eventing system
by looking at our event display function logs.

```shell
kubectl -n default get pods
kubectl -n default logs drive-event-display-XXXX user-container
```

You should see log lines similar to:

```
☁️  CloudEvent: valid ✅
Context Attributes,
  SpecVersion: 0.2
  Type: org.nachocano.source.gsuite.drive
  Source: https://www.googleapis.com/drive/v3/changes?alt=json&includeCorpusRemovals=false&includeItemsFromAllDrives=false&includeRemoved=true&includeTeamDriveItems=false&pageSize=100&pageToken=30&prettyPrint=false&restrictToMyDrive=false&spaces=drive&supportsAllDrives=false&supportsTeamDrives=false&alt=json
  ID: r0RAXpKrtrXii0Dgu56Cx666dnM
  Time: 2019-04-30T07:29:09.830080957Z
  ContentType: application/json
  Extensions:
    goog: map[resource-id:["r0RAXpKrtrXii0Dgu56Cx666dnM"]]
Transport Context,
  URI: /
  Host: drive-event-display.default.svc.cluster.local
  Method: POST
Data,
  ""
```

### Cleanup

You can remove the `DriveSource` webhook by deleting the Source:

```shell
kubectl -n default delete drivesources drive-source-sample
```

