/*
Copyright 2026 Flant JSC

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

package processed_status

import (
	"crypto/md5"
	"encoding/hex"
	"os"
	"sort"
	"time"
)

func GetTimestamp() string {
	curTime := time.Now()
	if timeStr, ok := os.LookupEnv("TEST_CONDITIONS_CALC_NOW_TIME"); ok {
		curTime, _ = time.Parse(time.RFC3339, timeStr)
	}
	return curTime.Format(time.RFC3339)
}

func CalculateChecksum(values ...string) string {
	if env, ok := os.LookupEnv("TEST_CONDITIONS_CALC_CHKSUM"); ok {
		return env
	}

	sort.Strings(values)
	hasher := md5.New()
	for _, value := range values {
		_, _ = hasher.Write([]byte(value))
	}
	return hex.EncodeToString(hasher.Sum(nil))
}
