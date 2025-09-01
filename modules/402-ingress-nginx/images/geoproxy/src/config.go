/*
Copyright 2025 Flant JSC

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

package geodownloader

import (
	"fmt"
	"os"
	"time"

	"github.com/deckhouse/deckhouse/pkg/log"
)

//	 type AccountMaxmind struct {
//		MaxmindAccount    int
//		MaxmindLicenseKey string
//		MaxmindEditionsDB string
//	 }
type Config struct {
	MaxmindIntervalUpdate time.Duration
}

var (
	PathDb = "/data"
)

func NewConfig() *Config {
	intervalUpdate := os.Getenv("INTERVAL_UPDATE")
	interval, err := time.ParseDuration(intervalUpdate)
	if err != nil {
		interval = time.Minute * 60
		log.Error(fmt.Sprintf("error parsing INTERVAL_UPDATE: %v", err))
	}

	return &Config{
		MaxmindIntervalUpdate: interval,
	}
}
