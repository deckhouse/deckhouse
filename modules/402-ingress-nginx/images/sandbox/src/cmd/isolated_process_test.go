/*
Copyright 2026 Flant JSC

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

package main

import (
	"testing"

	"golang.org/x/sys/unix"
)

func TestCapabilityWordMask(t *testing.T) {
	tests := []struct {
		name     string
		cap      uint
		wantWord int
		wantMask uint32
		wantErr  bool
	}{
		{
			name:     "capability in first word",
			cap:      unix.CAP_SYS_CHROOT,
			wantWord: 0,
			wantMask: uint32(1) << (unix.CAP_SYS_CHROOT % 32),
		},
		{
			name:     "capability in second word",
			cap:      33,
			wantWord: 1,
			wantMask: uint32(1) << 1,
		},
		{
			name:    "capability out of range",
			cap:     64,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotWord, gotMask, err := capabilityWordMask(tt.cap)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if gotWord != tt.wantWord {
				t.Fatalf("unexpected word, got %d want %d", gotWord, tt.wantWord)
			}
			if gotMask != tt.wantMask {
				t.Fatalf("unexpected mask, got %032b want %032b", gotMask, tt.wantMask)
			}
		})
	}
}
