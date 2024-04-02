// Copyright 2024 Flant JSC
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

package debug

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"

	"gopkg.in/alecthomas/kingpin.v2"
)

func DefineRequirementsCommands(kpApp *kingpin.Application) {
	requirementsCmd := kpApp.Command("requirements", "Dump all requirements from memory storage.")
	requirementsCmd.Action(func(c *kingpin.ParseContext) error {
		debugServerAddr := os.Getenv("DEBUG_HTTP_SERVER_ADDR")
		resp, err := http.Get(fmt.Sprintf("http://%s/requirements", debugServerAddr))
		if err != nil || resp.StatusCode != http.StatusOK {
			return fmt.Errorf("error getting requirements")
		}
		defer resp.Body.Close()

		var requirements = make(map[string]interface{})
		decoder := json.NewDecoder(resp.Body)
		if err := decoder.Decode(&requirements); err != nil {
			return fmt.Errorf("error unmarshal requirements")
		}

		for key, value := range requirements {
			fmt.Printf("%v: %v\n", key, value)
		}

		return nil
	})
}
