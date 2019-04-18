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
	"github.com/cloudevents/sdk-go/pkg/cloudevents"
	"github.com/cloudevents/sdk-go/pkg/cloudevents/client"
	"github.com/cloudevents/sdk-go/pkg/cloudevents/types"
	"github.com/knative/eventing-sources/pkg/kncloudevents"
	sourcesv1alpha1 "github.com/nachocano/gsuite-source/pkg/apis/sources/v1alpha1"
	gscalendar "google.golang.org/api/calendar/v3"
	"k8s.io/apimachinery/pkg/util/uuid"
	"log"
	"net/http"
	"sync"
)

type Adapter struct {
	sink string
	port string

	ceClient       client.Client
	initClientOnce sync.Once

	service *gscalendar.Service
	token   string
	id      string
}

func New(ctx context.Context, sink, port string) (*Adapter, error) {
	a := new(Adapter)
	var err error

	a.ceClient, err = kncloudevents.NewDefaultClient(sink)
	if err != nil {
		return nil, err
	}

	a.service, err = gscalendar.NewService(ctx)
	if err != nil {
		return nil, err
	}
	a.sink = sink
	a.port = port
	a.id = string(uuid.NewUUID())
	a.token = string(uuid.NewUUID())
	return a, nil
}

func (a *Adapter) Watch() error {
	channel := &gscalendar.Channel{
		Id:      a.id,
		Token:   a.token,
		Address: "",
		Payload: true,
		Kind:    "api#channel",
	}
	resp, err := a.service.CalendarList.Watch(channel).Do()
	if err != nil {
		return err
	}
	log.Printf("Id %s", resp.Id)
	log.Printf("Token %s", resp.Token)
	log.Printf("Kind %s", resp.Kind)
	log.Printf("Type %s", resp.Type)
	return nil
}

func (a *Adapter) HandleEvent(payload interface{}, header http.Header) {
	hdr := http.Header(header)
	err := a.handleEvent(payload, hdr)
	if err != nil {
		log.Printf("unexpected error handling Calendar event: %s", err)
	}
}

func (a *Adapter) handleEvent(payload interface{}, hdr http.Header) error {
	var err error
	a.initClientOnce.Do(func() {
		a.ceClient, err = kncloudevents.NewDefaultClient(a.sink)
	})
	if a.ceClient == nil {
		return fmt.Errorf("failed to create cloudevent client: %s", err)
	}

	cloudEventType := fmt.Sprintf("%s.%s", sourcesv1alpha1.CalendarSourceEventPrefix, "type")
	subject := subjectFromCalendarEvent("", payload)
	if err != nil {
		return err
	}

	eventContext := cloudevents.EventContextV02{
		ID:     "",
		Type:   cloudEventType,
		Source: *types.ParseURLRef("/CalendarSource"),
	}.AsV02()
	eventContext.SetSubject(subject)

	event := cloudevents.Event{
		Context: eventContext,
		Data:    payload,
	}

	_, err = a.ceClient.Send(context.TODO(), event)
	return err
}

func subjectFromCalendarEvent(calendarEvent string, payload interface{}) string {
	return "/calendar-demo"
}
