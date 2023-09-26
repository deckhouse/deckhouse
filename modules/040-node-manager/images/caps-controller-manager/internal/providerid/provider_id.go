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

package providerid

import (
	"fmt"
	"github.com/pkg/errors"
	"regexp"
	"sigs.k8s.io/cluster-api/util"
)

const (
	// Prefix is the prefix for a static node provider ID.
	Prefix = "static://"
)

type ProviderID string

// GenerateProviderID generates a provider ID for a static node.
func GenerateProviderID() ProviderID {
	return ProviderID(fmt.Sprintf("%s/%s", Prefix, util.RandomString(16)))
}

// ValidateProviderID validates a provider ID for a static node.
func ValidateProviderID(providerID ProviderID) error {
	match, err := regexp.MatchString(fmt.Sprintf("%s/.+", Prefix), string(providerID))
	if err != nil {
		return err
	}
	if match {
		return nil
	}

	return errors.New("invalid format for provider id")
}
