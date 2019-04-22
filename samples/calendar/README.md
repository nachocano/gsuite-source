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

## 

## 