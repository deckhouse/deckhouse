/*
Copyright 2022 Flant JSC

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

type ConversionFunc func(settings *Settings) error

type Conversion struct {
	Source     int
	Target     int
	Conversion ConversionFunc
}

func (c *Conversion) Convert(settings *Settings) (*Settings, error) {
	if c.Conversion == nil {
		return nil, nil
	}
	// Copy values to prevent accidental mutating on error.
	newValues := SettingsFromBytes(settings.Bytes())
	err := c.Conversion(newValues)
	if err != nil {
		return nil, err
	}
	return newValues, nil
}

func NewConversion(srcVersion int, targetVersion int, conversionFunc ConversionFunc) *Conversion {
	return &Conversion{
		Source:     srcVersion,
		Target:     targetVersion,
		Conversion: conversionFunc,
	}
}
