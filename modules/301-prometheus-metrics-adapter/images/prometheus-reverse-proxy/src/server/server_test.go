/*
Copyright 2022 Flant JSC

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

package server

import (
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"testing"
	"time"
)

type testServer struct {
	address string
}

func healthzProbe(address string) bool {
	req, err := http.NewRequest("GET", address+"/healthz", nil)
	if err != nil {
		return false
	}

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		time.Sleep(time.Microsecond)
		return false
	}

	defer res.Body.Close()

	b, err := ioutil.ReadAll(res.Body)
	if err != nil {
		time.Sleep(time.Microsecond)
		return false
	}

	return string(b) == "Ok."
}

func setupTestServer(t *testing.T) *testServer {
	config = map[string]map[string]CustomMetricConfig{
		"my_kind": {
			"my_metric": CustomMetricConfig{
				Namespaced: map[string]string{
					"default": "sum by (<<.GroupBy>>) (metric_name{<<.LabelMatchers>>})",
				},
			},
		},
	}

	prometheusServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, r.URL.String())
	}))

	if err := os.Setenv("PROMETHEUS_URL", prometheusServer.URL); err != nil {
		t.Fatalf(err.Error())
	}

	// Search free address, then immediately close the listener to bind the server to this port instead
	listener, err := net.Listen("tcp", "localhost:0")
	if err != nil {
		t.Fatalf(err.Error())
	}

	address := listener.Addr()
	if err := listener.Close(); err != nil {
		t.Fatalf(err.Error())
	}

	if err := os.Setenv("PROXY_LISTEN_ADDRESS", address.String()); err != nil {
		t.Fatalf(err.Error())
	}

	go NewServer().Listen()
	ts := &testServer{
		address: "http://" + address.String(),
	}

	for i := 0; i < 100; i++ {
		if healthzProbe(ts.address) {
			break
		}
	}

	return ts
}

func TestServerHandleCustomMetrics(t *testing.T) {
	ts := setupTestServer(t)
	requestQuery := `/api/v1/query?query=` + url.QueryEscape(`custom_metric::my_kind::my_metric::namespace="default"::test`)

	req, err := http.NewRequest("GET", ts.address+requestQuery, nil)
	if err != nil {
		t.Fatal(err)
	}

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal("wanted a non-nil error, got " + err.Error())
	}

	defer res.Body.Close()

	b, err := ioutil.ReadAll(res.Body)
	if err != nil {
		t.Fatal(err)
	}

	if string(b) != `/api/v1/query?query=`+url.QueryEscape(`sum by (test) (metric_name{namespace="default"})`) {
		t.Fatalf("expected %q to be equal to %q", string(b), requestQuery)
	}
}

func TestServerProxyPass(t *testing.T) {
	ts := setupTestServer(t)
	requestQuery := "/api/v1/query"

	req, err := http.NewRequest("GET", ts.address+requestQuery, nil)
	if err != nil {
		t.Fatal(err)
	}

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal("wanted a non-nil error, got " + err.Error())
	}

	defer res.Body.Close()

	b, err := ioutil.ReadAll(res.Body)
	if err != nil {
		t.Fatal(err)
	}

	if string(b) != requestQuery {
		t.Fatalf("expected %q to be equal to %q", string(b), requestQuery)
	}
}
