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

package main

import (
	"encoding/json"
	"fmt"
	"github.com/VictoriaMetrics/metrics"
	"io/ioutil"
	"log"
	"net/http"
	"time"
)

type Target struct {
	LastError  string            `json:"lastError"`
	ScrapePool string            `json:"scrapePool"`
	Labels     map[string]string `json:"labels"`
}

type data struct {
	Target []Target `json:"activeTargets"`
}

type Prom struct {
	Data data `json:"data"`
}

func recordMetrics() {
	go func() {
		for {
			client := &http.Client{}
			req, err := http.NewRequest("GET", "http://127.0.0.1:9090/api/v1/targets?state=active&scrapePool=", nil)
			if err != nil {
				log.Println("1")
				log.Println(err)
			} else {
				resp, err := client.Do(req)
				if err != nil {
					log.Println("connect ", err)
				} else {
					defer resp.Body.Close()
					bodyText, err := ioutil.ReadAll(resp.Body)
					if err != nil {
						log.Println("3")
						log.Println(err)
					} else {
						var y Prom

						if err := json.Unmarshal([]byte(string(bodyText)), &y); err != nil {
							log.Println("json parse", err)
							log.Println(string(bodyText))
						} else {
							metrics.UnregisterAllMetrics()
							for row := range y.Data.Target {
								if y.Data.Target[row].LastError == "sample limit exceeded" {
									s := fmt.Sprintf(`scrapePool="%s"`, y.Data.Target[row].ScrapePool)
									for k, v := range y.Data.Target[row].Labels {
										s += fmt.Sprintf(`,%s="%s"`, k, v)
									}
									name := fmt.Sprintf(`prometheus_target_limits_metrics{%s}`, s)
									metrics.GetOrCreateCounter(name).Add(1)
								}
							}
						}
					}
				}
			}
			time.Sleep(600 * time.Second)
		}
	}()
}

func main() {
	log.Println("start recordMetrics()")
	recordMetrics()
	log.Println("start http.HandleFunc()")
	http.HandleFunc("/metrics", func(w http.ResponseWriter, req *http.Request) {
		metrics.WritePrometheus(w, false)
	})
	log.Println("start http.ListenAndServe()")
	http.ListenAndServe(":9101", nil)
}
