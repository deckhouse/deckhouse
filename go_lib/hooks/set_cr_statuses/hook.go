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
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	utils_checksum "github.com/flant/shell-operator/pkg/utils/checksum"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func getTimeStamp() string {
	curTime := time.Now()
	if timeStr, ok := os.LookupEnv("TEST_CONDITIONS_CALC_NOW_TIME"); ok {
		curTime, _ = time.Parse(time.RFC3339, timeStr)
	}
	return curTime.Format(time.RFC3339)
}

func getCheckSum(bytes []byte) string {
	checkSum := utils_checksum.CalculateChecksum(string(bytes))
	if env, ok := os.LookupEnv("TEST_CONDITIONS_CALC_CHKSUM"); ok {
		checkSum = env
	}
	return checkSum
}

func SetObservedStatus(snapshot go_hook.FilterResult, filterFunc func(*unstructured.Unstructured) (go_hook.FilterResult, error)) func(obj *unstructured.Unstructured) (*unstructured.Unstructured, error) {
	snBytes, _ := json.Marshal(snapshot)
	checkSum := getCheckSum(snBytes)

	return func(obj *unstructured.Unstructured) (*unstructured.Unstructured, error) {
		objCopy := obj.DeepCopy()
		filteredObj, err := filterFunc(objCopy)
		if err != nil {
			return nil, fmt.Errorf("cannot apply filterFunc to object: %v", err)
		}

		objBytes, err := json.Marshal(filteredObj)
		if err != nil {
			return nil, fmt.Errorf("cannot marshal filtered object: %v", err)
		}

		objCheckSum := getCheckSum(objBytes)
		if checkSum == objCheckSum {
			processedCheckSum, found, err := unstructured.NestedString(objCopy.Object, "status", "deckhouse", "processed", "checkSum")
			if err != nil {
				return nil, fmt.Errorf("cannot get processed checksum status field: %v", err)
			}

			if !found || checkSum != processedCheckSum {
				if err := unstructured.SetNestedField(objCopy.Object, "False", "status", "deckhouse", "synced"); err != nil {
					return nil, fmt.Errorf("cannot set synced status field: %v", err)
				}
			} else {
				if err := unstructured.SetNestedField(objCopy.Object, "True", "status", "deckhouse", "synced"); err != nil {
					return nil, fmt.Errorf("cannot set synced status field: %v", err)
				}
			}

			if err := unstructured.SetNestedStringMap(objCopy.Object, map[string]string{"lastTimestamp": getTimeStamp(), "checkSum": objCheckSum}, "status", "deckhouse", "observed"); err != nil {
				return nil, fmt.Errorf("cannot set observed status field: %v", err)
			}
		} else {
			return nil, fmt.Errorf("object has changed since last snapshot")
		}
		return objCopy, nil
	}
}

func SetProcessedStatus(filterFunc func(*unstructured.Unstructured) (go_hook.FilterResult, error)) func(obj *unstructured.Unstructured) (*unstructured.Unstructured, error) {
	return func(obj *unstructured.Unstructured) (*unstructured.Unstructured, error) {
		objCopy := obj.DeepCopy()
		filteredObj, err := filterFunc(objCopy)
		if err != nil {
			return nil, fmt.Errorf("cannot apply filterFunc to object: %v", err)
		}

		objBytes, err := json.Marshal(filteredObj)
		if err != nil {
			return nil, fmt.Errorf("cannot marshal filtered object: %v", err)
		}

		objCheckSum := getCheckSum(objBytes)

		observedCheckSum, found, err := unstructured.NestedString(objCopy.Object, "status", "deckhouse", "observed", "checkSum")
		if err != nil {
			return nil, fmt.Errorf("cannot get observed checksum status field: %v", err)
		}

		if !found || objCheckSum != observedCheckSum {
			if err := unstructured.SetNestedField(objCopy.Object, "False", "status", "deckhouse", "synced"); err != nil {
				return nil, fmt.Errorf("cannot set synced status field: %v", err)
			}
		} else {
			if err := unstructured.SetNestedField(objCopy.Object, "True", "status", "deckhouse", "synced"); err != nil {
				return nil, fmt.Errorf("cannot set synced status field: %v", err)
			}
		}

		if err := unstructured.SetNestedStringMap(objCopy.Object, map[string]string{"lastTimestamp": getTimeStamp(), "checkSum": objCheckSum}, "status", "deckhouse", "processed"); err != nil {
			return nil, fmt.Errorf("cannot set processed status field: %v", err)
		}
		return objCopy, nil
	}
}
