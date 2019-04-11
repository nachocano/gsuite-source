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
	"github.com/knative/pkg/logging"
	sourcesv1alpha1 "github.com/nachocano/gsuite-source/pkg/apis/sources/v1alpha1"
	"go.uber.org/zap"
	"log"
	"net/http"
	"sync"
)

type Adapter struct {
	Sink   string
	client client.Client

	initClientOnce sync.Once
}

func New(sinkURI string) (*Adapter, error) {
	a := new(Adapter)
	var err error
	a.client, err = kncloudevents.NewDefaultClient(sinkURI)
	if err != nil {
		return nil, err
	}
	return a, nil
}

func (a *Adapter) Start(ctx context.Context, stopCh <-chan struct{}) error {
	logger := logging.FromContext(ctx)

	logger.Info("Starting with config: ", zap.Any("adapter", a))

	for {
		select {
		case <-stopCh:
			logger.Info("Exiting")
			return nil
		default:
		}
	}
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
		a.client, err = kncloudevents.NewDefaultClient(a.Sink)
	})
	if a.client == nil {
		return fmt.Errorf("failed to create cloudevent client: %s", err)
	}

	cloudEventType := fmt.Sprintf("%s.%s", sourcesv1alpha1.SheetsSourceEventPrefix, "type")
	source, err := sourceFromCalendarEvent("", payload)
	if err != nil {
		return err
	}

	event := cloudevents.Event{
		Context: cloudevents.EventContextV02{
			ID:     "",
			Type:   cloudEventType,
			Source: *source,
		}.AsV02(),
		Data: payload,
	}
	_, err = a.client.Send(context.TODO(), event)
	return err
}

func sourceFromCalendarEvent(sheetsEvent string, payload interface{}) (*types.URLRef, error) {
	url := "/calendar-demo"
	if url != "" {
		source := types.ParseURLRef(url)
		if source != nil {
			return source, nil
		}
	}

	return nil, fmt.Errorf("no source found in sheets event %q", sheetsEvent)
}
