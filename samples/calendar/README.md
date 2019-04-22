# Google Calendar Source 

This sample shows how to wire Google Calendar events into Knative Eventing.

## Prerequisites

You will need:

1. Follow these [prerequisites](https://github.com/nachocano/gsuite-source#prerequisites).
1. Enable Google Calendar API in your GCP project by executing the following command: 
    ```shell
    gcloud services enable calendar-json.googleapis.com
    ```
1. Register your domain to be able to receive push notifications. Follow [these](https://developers.google.com/calendar/v3/push#registering-your-domain) steps.
1. Delegate domain-wide authority to your service account. 
Follow [these](https://developers.google.com/admin-sdk/directory/v1/guides/delegation#delegate_domain-wide_authority_to_your_service_account) steps, and
    1. When specifying the API scopes, enter the calendar scope: `https://www.googleapis.com/auth/calendar`. 
    1. When asked for the Client ID, enter the your service account's one that you saved during the previous prerequisites.

## Details
The actual implementation contacts the Google Calendar API in order to create a 
channel, which is basically a web hook, to receive Push Notifications on Calendar event changes. 
The authentication is delegated to the service account, thus no user involvement is required.    
The notifications are delivered to a Knative Service (listening on an HTTPS public address), which converts  
the messages into [CloudEvents](https://github.com/cloudevents/spec) and forwards them to the configured sink.

## Calendar Source Spec Fields

The `CalendarSource` watches for [Calendar event](https://developers.google.com/calendar/v3/reference/events/watch) changes. 
Here are its `spec` fields:

- `emailAddress`: `string` The user email address corresponding to the calendar events we are interested in. Must be set.

- `gcpCredsSecret`: a [SecretKeySelector](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.12/#secretkeyselector-v1-core)
  containing the service account secret that will take care of the authentication. Must be set.
- `sink`:
  [ObjectReference](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.12/#objectreference-v1-core)
  A reference to the object that should receive events. Must be set.

## Example

Now we are going to show an example of how to consume Calendar events.

### Create a Knative Service

To verify the `CalendarSource` is working, we will create a simple Knative Service that dumps incoming messages to its log. 
The `service.yaml` file defines this basic service.

```yaml
apiVersion: serving.knative.dev/v1alpha1
kind: Service
metadata:
  name: calendar-event-display
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

### Create an Event Source for Calendar Events

In order to receive Calendar events, you have to create a concrete 
`CalendarSource` CR in a specific namespace. Be sure to replace the
`emailAddress` value with a valid email address in your G Suite domain.

```yaml
apiVersion: sources.nachocano.org/v1alpha1
kind: CalendarSource
metadata:
  name: calendar-source-sample
spec:
  emailAddress: <YOUR EMAIL ADDRESS>
  gcpCredsSecret:
    name: gs-source-key
    key: key.json
  sink:
    apiVersion: serving.knative.dev/v1alpha1
    kind: Service
    name: calendar-event-display
```

Then, apply that yaml using `kubectl`:

```shell
kubectl -n default apply -f calendar-source.yaml
```

### Verify

Verify that the `CalendarSource` is ready by executing the following command:

```shell
kubectl get calendarsources
```
```
NAME                     READY   REASON
calendar-source-sample   True
```

### Create Events

Create a calendar event in the user's email address Calendar application. 
We will verify that the Calendar event was sent to the Knative eventing system
by looking at our event display function logs.

```shell
kubectl -n default get pods
kubectl -n default logs calendar-event-display-XXXX user-container
```

You should see log lines similar to:

```
☁️  CloudEvent: valid ✅
Context Attributes,
  SpecVersion: 0.2
  Type: org.nachocano.source.gsuite.calendar
  Source: https://www.googleapis.com/calendar/v3/calendars/primary/events?alt=json&maxResults=250&prettyPrint=false&alt=json
  ID: ExEtu74ipgEsOKwJEmos06HzMSI
  Time: 2019-04-22T05:53:53.646880605Z
  ContentType: application/json
  Extensions:
    goog: map[resource-id:["ExEtu74ipgEsOKwJEmos06HzMSI"]]
Transport Context,
  URI: /
  Host: calendar-event-display.default.svc.cluster.local
  Method: POST
Data,
  ""
```

### Cleanup

You can remove the `CalendarSource` webhook by deleting the Source:

```shell
kubectl -n default delete calendarsources calendar-source-sample
```

## Limitations & Known Issues

1. Currently, Calendar does not include a message body in their [Push Notifications](https://developers.google.com/calendar/v3/push). 
Push Notifications are just a way of notifying that some watched resource changed in order to avoid unnecessary polling. 
However, *which* particular resource changed cannot be determined from the notification, therefore, a Calendar API call 
to retrieve the updated list of resources is needed. We are not performing that call as of now. 
1. The notification channel expires after a configurable period of time (by default 1 hour). We are not renewing that channel. 
1. Only listens to events from the *primary* calendar of the specified `emailAddress` account. 
1. Only a single email address can be specified.
1. If there is a problem updating the status of the `CalendarSource`, more than one web hook might be created. 