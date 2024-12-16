/*
Copyright 2021 Flant JSC

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

package errors

import (
	"errors"
	"fmt"
	"strings"

	"github.com/fatih/color"
	"github.com/kyokomi/emoji"
)

type LintRuleError struct {
	Text     string
	ID       string
	ObjectID string
	Value    interface{}
}

func (l *LintRuleError) EqualsTo(candidate LintRuleError) bool {
	return l.ID == candidate.ID && l.Text == candidate.Text && l.ObjectID == candidate.ObjectID
}

func (l *LintRuleError) IsEmpty() bool {
	return l.ID == "" && l.Text == "" && l.ObjectID == ""
}

func NewLintRuleError(id, objectID string, value interface{}, template string, a ...interface{}) LintRuleError {
	return LintRuleError{
		ObjectID: objectID,
		Value:    value,
		Text:     fmt.Sprintf(template, a...),
		ID:       id,
	}
}

var EmptyRuleError = LintRuleError{Text: "", ID: "", ObjectID: ""}

type LintRuleErrorsList struct {
	data []LintRuleError
}

func (l *LintRuleErrorsList) Add(e LintRuleError) {
	if e.IsEmpty() {
		return
	}
	for _, addedError := range l.data {
		if e.EqualsTo(addedError) {
			return
		}
	}
	l.data = append(l.data, e)
}

func (l *LintRuleErrorsList) Merge(e LintRuleErrorsList) {
	l.data = append(l.data, e.data...)
}

func (l *LintRuleErrorsList) ConvertToError() error {
	if len(l.data) == 0 {
		return nil
	}

	builder := strings.Builder{}
	for _, err := range l.data {
		builder.WriteString(fmt.Sprintf(
			"%s%s\n\tMessage\t- %s\n\tObject\t- %s\n",
			emoji.Sprintf(":monkey:"),
			color.New(color.FgHiBlue).SprintfFunc()("[#%s]", err.ID),
			color.New(color.FgRed).SprintfFunc()(err.Text),
			err.ObjectID,
		))

		if err.Value != nil {
			builder.WriteString(fmt.Sprintf("\tValue\t- %v\n", err.Value))
		}
		builder.WriteString("\n")
	}
	return errors.New(builder.String())
}
