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
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"strings"
	"sync"
)

const (
	calendarHeaderResourceID    = "Goog-Resource-ID"
	calendarHeaderResourceURI   = "Goog-Resource-URI"
	calendarHeaderChannelToken  = "Goog-Channel-Token"
	calendarHeaderResourceState = "Goog-Resource-State"
)

type Adapter struct {
	sink string

	ceClient       client.Client
	initClientOnce sync.Once

	token string
}

func New(sink string) (*Adapter, error) {
	a := new(Adapter)
	var err error
	a.sink = sink
	a.ceClient, err = kncloudevents.NewDefaultClient(sink)
	if err != nil {
		return nil, err
	}
	return a, nil
}

func (a *Adapter) ParseEvent(r *http.Request) (interface{}, error) {
	defer func() {
		_, _ = io.Copy(ioutil.Discard, r.Body)
		_ = r.Body.Close()
	}()

	if r.Method != http.MethodPost {
		return nil, fmt.Errorf("invalid HTTP Method %s", r.Method)
	}

	token := r.Header.Get("X-" + calendarHeaderChannelToken)
	if token == "" {
		return nil, fmt.Errorf("missing X-%s header", calendarHeaderChannelToken)
	}
	if token != sourcesv1alpha1.CalendarSourceToken {
		return nil, fmt.Errorf("token mismatch, want %q, got %q", sourcesv1alpha1.CalendarSourceToken, token)
	}

	if strings.EqualFold("sync", r.Header.Get("X-"+calendarHeaderResourceState)) {
		return nil, fmt.Errorf("sync message received")
	}

	// TODO payloads always empty, remove this.
	payload, err := ioutil.ReadAll(r.Body)
	if err != nil {
		return nil, fmt.Errorf("error parsing payload: %v", err)
	}
	// len(payload) == 0
	return payload, nil
}

func (a *Adapter) HandleEvent(payload interface{}, header http.Header) {
	hdr := http.Header(header)
	err := a.handleEvent(payload, hdr)
	if err != nil {
		log.Printf("unexpected error handling calendar event: %v", err)
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

	eventId := hdr.Get("X-" + calendarHeaderResourceID)
	source := hdr.Get("X-" + calendarHeaderResourceURI)
	extensions := map[string]interface{}{
		calendarHeaderResourceID: eventId,
	}

	log.Printf("ResourceId %s", eventId)
	log.Printf("Source %s", source)
	log.Printf("Expiration %s", hdr.Get("X-Goog-Channel-Expiration"))

	//if payload != nil {
	//	log.Printf("Payload %s", payload)
	//}

	eventContext := cloudevents.EventContextV02{
		ID:         eventId,
		Type:       sourcesv1alpha1.CalendarSourceEventType,
		Source:     *types.ParseURLRef(source),
		Extensions: extensions,
	}.AsV02()

	event := cloudevents.Event{
		Context: eventContext,
		Data:    payload,
	}

	_, err = a.ceClient.Send(context.TODO(), event)
	return err
}
