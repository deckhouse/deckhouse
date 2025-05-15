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

package utils

import (
	"fmt"

	"k8s.io/apimachinery/pkg/api/resource"
)

func ConvertStringQuantityToInt64(quantityStr string) (int64, error) {
	if quantityStr == "" {
		return 0, fmt.Errorf("quantity string can't be empty")
	}

	quantity, err := resource.ParseQuantity(quantityStr)
	if err != nil {
		return 0, err
	}

	result, ok := quantity.AsInt64()
	if !ok {
		return 0, fmt.Errorf("quantity %s can't be converted to int64", quantityStr)
	}
	return result, nil
}

func ConvertInt64ToStringQuantity(param int64) string {
	quantity := resource.NewQuantity(param, resource.BinarySI)
	return quantity.String()
}
