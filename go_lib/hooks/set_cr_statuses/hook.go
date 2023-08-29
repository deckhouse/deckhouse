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

package set_cr_statuses

import (
	"fmt"
	"os"
	"time"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func getTimeStamp() string {
	curTime := time.Now()
	if timeStr, ok := os.LookupEnv("TEST_CONDITIONS_CALC_NOW_TIME"); ok {
		curTime, _ = time.Parse(time.RFC3339, timeStr)
	}
	return curTime.Format(time.RFC3339)
}

var SetProcessedStatus = func(obj *unstructured.Unstructured) (*unstructured.Unstructured, error) {
	objCopy := obj.DeepCopy()
	if err := unstructured.SetNestedField(objCopy.Object, objCopy.GetGeneration(), "status", "deckhouse", "processed", "generation"); err != nil {
		return nil, fmt.Errorf("cannot set generation status field: %v", err)
	}

	if err := unstructured.SetNestedField(objCopy.Object, getTimeStamp(), "status", "deckhouse", "processed", "lastTimestamp"); err != nil {
		return nil, fmt.Errorf("cannot set generation status field: %v", err)
	}

	return objCopy, nil
}
