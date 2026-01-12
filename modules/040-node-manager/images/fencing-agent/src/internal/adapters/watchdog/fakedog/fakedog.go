/*
Copyright 2024 Flant JSC

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

package fakedog

import "fmt"

type WatchDog struct {
	signals *[]byte
	isArmed bool
}

func NewWatchdog(s *[]byte) *WatchDog {
	return &WatchDog{
		signals: s,
	}
}

func (w *WatchDog) IsArmed() bool {
	return w.isArmed
}

func (w *WatchDog) Start() error {
	if w.isArmed {
		return fmt.Errorf("Fakedog already armed")
	}
	w.isArmed = true
	return nil
}

func (w *WatchDog) Feed() error {
	if !w.isArmed {
		return fmt.Errorf("Fakedog is not armed")
	}
	*w.signals = append(*w.signals, 0)
	return nil
}

func (w *WatchDog) Stop() error {
	if !w.isArmed {
		return fmt.Errorf("Fakedog already disarmed")
	}
	*w.signals = append(*w.signals, 1)
	w.isArmed = false
	return nil
}
