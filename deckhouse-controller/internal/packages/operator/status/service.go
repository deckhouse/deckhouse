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

package status

import (
	"errors"
	"sync"
)

const (
	ConditionDownloaded        ConditionName = "Downloaded"
	ConditionReadyOnFilesystem ConditionName = "ReadyOnFilesystem"
	ConditionRequirementsMet   ConditionName = "RequirementsMet"
	ConditionReadyInRuntime    ConditionName = "ReadyInRuntime"
	ConditionHooksProcessed    ConditionName = "HooksProcessed"
	ConditionHelmApplied       ConditionName = "HelmApplied"
)

type Error struct {
	Err        error
	Conditions []Condition
}

func (e *Error) Error() string {
	return e.Err.Error()
}

type ConditionName string

type ConditionReason string

type Service struct {
	mu       sync.Mutex
	statuses map[string]*Status

	ch chan string
}

type Status struct {
	Conditions []Condition `json:"conditions" yaml:"conditions"`
}

type Condition struct {
	Name    ConditionName   `json:"name" yaml:"name"`
	Status  bool            `json:"status" yaml:"status"`
	Reason  ConditionReason `json:"reason,omitempty" yaml:"reason,omitempty"`
	Message string          `json:"message,omitempty" yaml:"message,omitempty"`
}

func NewService() *Service {
	return &Service{
		ch:       make(chan string, 10000),
		statuses: make(map[string]*Status),
	}
}

func (s *Service) GetCh() <-chan string {
	return s.ch
}

func (s *Service) GetStatus(name string) *Status {
	s.mu.Lock()
	defer s.mu.Unlock()

	if status, ok := s.statuses[name]; ok {
		return status
	}

	return nil
}

func (s *Service) SetConditionTrue(name string, condition ConditionName) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.statuses[name]; !ok {
		s.statuses[name] = new(Status)
	}

	if s.statuses[name].setCondition(Condition{Name: condition, Status: true}) {
		s.ch <- name
	}
}

func (s *Service) HandleError(name string, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.statuses[name]; !ok {
		s.statuses[name] = new(Status)
	}

	var notify bool
	for _, condition := range extractConditions(err) {
		if s.statuses[name].setCondition(condition) {
			notify = true
		}
	}

	if notify {
		s.ch <- name
	}
}

func extractConditions(err error) []Condition {
	statusErr := new(Error)
	if !errors.As(err, &statusErr) {
		return nil
	}

	conds := extractConditions(statusErr.Err)
	if len(conds) > 0 {
		conds = append(statusErr.Conditions, conds...)
	}

	return conds
}

func (s *Status) setCondition(condition Condition) bool {
	var notify bool
	var found bool
	for i := range s.Conditions {
		if s.Conditions[i].Name != condition.Name {
			continue
		}

		found = true

		if s.Conditions[i].Status != condition.Status {
			s.Conditions[i].Status = condition.Status
			notify = true
		}

		if s.Conditions[i].Reason != condition.Reason {
			s.Conditions[i].Reason = condition.Reason
			notify = true
		}

		if s.Conditions[i].Message != condition.Message {
			s.Conditions[i].Message = condition.Message
			notify = true
		}
	}

	if !found {
		s.Conditions = append(s.Conditions, condition)
		notify = true
	}

	return notify
}
