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

package sheets

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"sync"

	"github.com/cloudevents/sdk-go/pkg/cloudevents"
	"github.com/cloudevents/sdk-go/pkg/cloudevents/client"
	"github.com/cloudevents/sdk-go/pkg/cloudevents/types"
	"github.com/knative/eventing-sources/pkg/kncloudevents"
	sourcesv1alpha1 "github.com/nachocano/gsuite-source/pkg/apis/sources/v1alpha1"
)

// Adapter converts incoming Sheets webhook events to CloudEvents and
// then sends them to the specified Sink.
type Adapter struct {
	Sink   string
	client client.Client

	initClientOnce sync.Once
}

// HandleEvent is invoked whenever an event comes in from Sheets.
func (a *Adapter) HandleEvent(payload interface{}, header http.Header) {
	hdr := http.Header(header)
	err := a.handleEvent(payload, hdr)
	if err != nil {
		log.Printf("unexpected error handling Sheets event: %s", err)
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
	source, err := sourceFromSheetsEvent("", payload)
	if err != nil {
		return err
	}

	event := cloudevents.Event{
		Context: cloudevents.EventContextV02{
			ID:         "",
			Type:       cloudEventType,
			Source:     *source,
		}.AsV02(),
		Data: payload,
	}
	_, err = a.client.Send(context.TODO(), event)
	return err
}

func sourceFromSheetsEvent(sheetsEvent string , payload interface{}) (*types.URLRef, error) {
	url := "/demo"
	if url != "" {
		source := types.ParseURLRef(url)
		if source != nil {
			return source, nil
		}
	}

	return nil, fmt.Errorf("no source found in sheets event %q", sheetsEvent)
}
