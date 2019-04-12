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

package main

import (
	"flag"
	"github.com/nachocano/gsuite-source/pkg/adapter/calendar"
	"go.uber.org/zap"
	"golang.org/x/net/context"
	"log"
	"net/http"
	"os"
	"sigs.k8s.io/controller-runtime/pkg/runtime/signals"
)

const (
	// Environment variable containing the HTTP port
	envPort = "PORT"
	// Environment variable containing the sink
	envSink = "SINK"
)

func main() {
	flag.Parse()

	ctx := context.Background()

	sink := os.Getenv(envSink)
	if sink == "" {
		log.Fatalf("No sink given")
	}

	port := os.Getenv(envPort)
	if port == "" {
		port = "8080"
	}

	ra, err := calendar.New(sink)
	if err != nil {
		log.Fatalf("Failed to create calendar adapter: %s", err.Error())
	}

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		log.Print("Received event")
		ra.HandleEvent("", r.Header)
	})

	log.Print("Listening...")

	http.ListenAndServe(port, nil)

	stopCh := signals.SetupSignalHandler()
	if err := ra.Start(ctx, stopCh); err != nil {
		log.Fatal("failed to Start adapter: ", zap.Error(err))
	}

}
