// Copyright 2025 Flant JSC
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

package main

import (
	"fmt"

	"gopkg.in/yaml.v3"
)

// matrixBlock represents one Gator test template block.
//
// Preferred YAML:
//
//   - name: pss-baseline-functional-cases
//     gatorBlock: pss-baseline-functional
//     template: ...
//     constraint: ...
//     cases: [ ... ]
//
// If gatorBlock is omitted, name is used as Gator tests[].name (small tests).
type matrixBlock struct {
	Name              string       `yaml:"name"`
	GatorBlock        string       `yaml:"gatorBlock"`
	DefaultObjectBase string       `yaml:"defaultObjectBase"`
	Template          string       `yaml:"template"`
	Constraint        string       `yaml:"constraint"`
	Cases             []matrixCase `yaml:"cases"`
}

func (b *matrixBlock) gatorTestBlockName() string {
	if b.GatorBlock != "" {
		return b.GatorBlock
	}
	return b.Name
}

func (b *matrixBlock) UnmarshalYAML(value *yaml.Node) error {
	if value.Kind != yaml.MappingNode {
		return fmt.Errorf("matrixBlock: expected mapping, got %v", value.Kind)
	}
	var wire struct {
		Name              string    `yaml:"name"`
		GatorBlock        string    `yaml:"gatorBlock"`
		DefaultObjectBase string    `yaml:"defaultObjectBase"`
		Template          string    `yaml:"template"`
		Constraint        string    `yaml:"constraint"`
		Cases             yaml.Node `yaml:"cases"`
	}
	if err := value.Decode(&wire); err != nil {
		return err
	}
	b.Template = wire.Template
	b.Constraint = wire.Constraint
	b.GatorBlock = wire.GatorBlock
	b.DefaultObjectBase = wire.DefaultObjectBase

	switch wire.Cases.Kind {
	case yaml.SequenceNode:
		b.Name = wire.Name
		if err := wire.Cases.Decode(&b.Cases); err != nil {
			return err
		}
		return nil
	case yaml.MappingNode:
		var grp struct {
			Name  string       `yaml:"name"`
			Items []matrixCase `yaml:"items"`
		}
		if err := wire.Cases.Decode(&grp); err != nil {
			return err
		}
		if grp.Name != "" {
			b.Name = grp.Name
		} else {
			b.Name = wire.Name
		}
		b.GatorBlock = wire.Name
		if wire.GatorBlock != "" {
			b.GatorBlock = wire.GatorBlock
		}
		b.Cases = grp.Items
		if len(b.Cases) == 0 {
			return fmt.Errorf("block %q: cases.items is empty", b.gatorTestBlockName())
		}
		return nil
	case yaml.Kind(0):
		return fmt.Errorf("cases is required (for block %q)", wire.Name)
	default:
		return fmt.Errorf("cases must be a list or a legacy { name, items } map")
	}
}
