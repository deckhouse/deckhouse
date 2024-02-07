/*
Copyright 2023 Flant JSC

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

package src

import (
	"log"
	"net/http"
	"sync"
)

var (
	healthCheckStatus bool
	mutex             sync.RWMutex
)

func InitHealtcheck() {
	http.HandleFunc("/healthz", healthCheckHandler)
	log.Fatal(http.ListenAndServe(":9743", nil))
}

// healthCheckHandler is the handler function for the liveness and readiness probes
func healthCheckHandler(w http.ResponseWriter, r *http.Request) {
	mutex.RLock()
	defer mutex.RUnlock()

	if healthCheckStatus {
		// Return HTTP 200 OK if the application is healthy
		w.WriteHeader(http.StatusOK)
	} else {
		// Return HTTP 500 Internal Server Error if the application is unhealthy
		w.WriteHeader(http.StatusInternalServerError)
	}
}

func SetHealthCheckStatus(status bool) {
	mutex.Lock()
	defer mutex.Unlock()

	healthCheckStatus = status
}
