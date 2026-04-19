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

package conversion

import (
	"flag"
)

type Conversion struct {
	fs *flag.FlagSet
}

func NewConversion() *Conversion {
	ic := &Conversion{
		fs: flag.NewFlagSet("conversion", flag.ContinueOnError),
	}

	return ic
}

func (ic *Conversion) Name() string {
	return ic.fs.Name()
}

func (ic *Conversion) Init(args []string) error {
	return ic.fs.Parse(args)
}

func (ic *Conversion) Run() error {
	return run()
}
