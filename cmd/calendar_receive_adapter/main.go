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
	"fmt"
	"github.com/nachocano/gsuite-source/pkg/adapter/calendar"
	"go.uber.org/zap"
	"log"
	"net/http"
	"os"
)

const (
	// Environment variable containing the HTTP port
	envPort = "PORT"
	// Environment variable containing the sink
	envSink = "SINK"
	// Environment variable containing the certificate file
	envCertificate = "CERTIFICATE"
	// Environment variable containing the private key file
	envPrivateKey = "PRIVATE_KEY"
)

func main() {
	flag.Parse()

	log.Print("Starting Calendar Adapter...")

	sink := os.Getenv(envSink)
	if sink == "" {
		log.Fatal("No sink given")
	}
	log.Printf("Sink %s", sink)

	port := os.Getenv(envPort)
	if port == "" {
		port = "8080"
	}
	log.Printf("Port %s", port)

	certFile := os.Getenv(envCertificate)
	if certFile == "" {
		log.Fatal("No certificate given")
	}
	log.Printf("Certificate File %s", certFile)

	keyFile := os.Getenv(envPrivateKey)
	if keyFile == "" {
		log.Fatal("No private key given")
	}
	log.Printf("Private Key File %s", keyFile)

	ra, err := calendar.New(sink)
	if err != nil {
		log.Fatalf("Failed to create Calendar Adapter: %v", zap.Error(err))
	}

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		log.Print("Event received")
		ra.HandleEvent(r.Body, r.Header)
	})

	addr := fmt.Sprintf(":%s", port)
	if err := http.ListenAndServe(addr, nil); err != nil {
		log.Fatalf("Failed to start Calendar Adapter: %v", zap.Error(err))
	}

	log.Print("Started Calendar Adapter")
}
