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
	"github.com/nachocano/gsuite-source/pkg/adapter/sheets"
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
)

func main() {
	sink := flag.String("sink", "", "uri to send events to")
	flag.Parse()

	ctx := context.Background()

	log.Printf("Starting sheets receive adapter with sink: %s", sink)

	if sink == nil || *sink == "" {
		log.Fatalf("No sink given")
	}

	port := os.Getenv(envPort)
	if port == "" {
		port = "8080"
	}

	log.Print("Creating server")
	ra, err := sheets.New(*sink)
	if err != nil {
		log.Fatalf("Failed to create sheets adapter: %s", err.Error())
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
