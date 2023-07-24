// Copyright 2023 Flant JSC
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

package app

import (
	"fmt"
	"regexp"

	"gopkg.in/alecthomas/kingpin.v2"
)

type stringWithRegexpValidation struct {
	value   string
	regexpr *regexp.Regexp
}

func (s *stringWithRegexpValidation) Set(value string) error {
	if match := s.regexpr.MatchString(value); !match {
		return fmt.Errorf("must match %s", s.regexpr)
	}
	s.value = value
	return nil
}

func (s *stringWithRegexpValidation) String() string {
	return s.value
}

func NewStringWithRegexpValidation(regexpr string) kingpin.Value {
	return &stringWithRegexpValidation{
		regexpr: regexp.MustCompile(regexpr),
	}
}
