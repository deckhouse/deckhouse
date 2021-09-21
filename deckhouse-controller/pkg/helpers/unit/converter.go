// Copyright 2021 Flant JSC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package unit

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"time"

	"k8s.io/apimachinery/pkg/api/resource"
)

func Convert(mode string, output string) error {
	reader := bufio.NewReader(os.Stdin)

	input, err := reader.ReadString('\n')
	if err != nil {
		return fmt.Errorf("failed to read stdin: %s", err)
	}
	input = strings.TrimSuffix(input, "\n")

	switch mode {
	case "duration":
		duration, err := time.ParseDuration(input)
		if err != nil {
			return fmt.Errorf("failed to parse: %s", err)
		}
		switch output {
		case "value":
			fmt.Println(duration.Seconds())
		case "milli":
			fmt.Println(duration.Milliseconds())
		}

	case "kube-resource-unit":
		quantity := resource.MustParse(input)
		switch output {
		case "value":
			fmt.Println(quantity.Value())
		case "milli":
			fmt.Println(quantity.MilliValue())
		}

	default:
		return fmt.Errorf("unknown mode")
	}
	return nil
}
