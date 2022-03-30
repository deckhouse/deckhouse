/*
Copyright 2021 Flant JSC

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

package util

import (
	"os"
	"strconv"
)

const (
	AgentUserAgent  = "UpmeterAgent/1.0"
	ServerUserAgent = "Upmeter/1.0"
)

func GetenvInt64(name string) int {
	s := os.Getenv(name)
	if s == "" || s == "0" {
		return 0
	}

	n, err := strconv.Atoi(s)
	if err != nil {
		return 0
	}
	return n
}
