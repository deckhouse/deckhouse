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

package authzgeneraterulesforroles

import (
	"flag"
)

type AuthzGenerate struct {
	fs *flag.FlagSet
}

func NewAuthzGenerate() *AuthzGenerate {
	ag := &AuthzGenerate{
		fs: flag.NewFlagSet("authz-generate-roles", flag.ContinueOnError),
	}

	return ag
}

func (ag *AuthzGenerate) Name() string {
	return ag.fs.Name()
}

func (ag *AuthzGenerate) Init(args []string) error {
	return ag.fs.Parse(args)
}

func (ag *AuthzGenerate) Run() error {
	run()

	return nil
}
