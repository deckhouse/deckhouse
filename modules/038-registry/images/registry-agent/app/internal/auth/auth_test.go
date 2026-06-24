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

package auth

import (
	"os"
	"path/filepath"
	"testing"

	"golang.org/x/crypto/bcrypt"
)

func TestAuthenticate(t *testing.T) {
	hash, err := bcrypt.GenerateFromPassword([]byte("s3cret"), bcrypt.DefaultCost)
	if err != nil {
		t.Fatal(err)
	}
	a := New([]User{{Name: "ro", PasswordHash: string(hash), Role: "ReadOnly"}})

	if !a.Authenticate("ro", "s3cret") {
		t.Fatal("valid creds rejected")
	}
	if a.Authenticate("ro", "wrong") {
		t.Fatal("wrong password accepted")
	}
	if a.Authenticate("nobody", "s3cret") {
		t.Fatal("unknown user accepted")
	}
	if a.Authenticate("ro", "") {
		t.Fatal("empty password accepted")
	}
}

func TestLoadUsers(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "users.yaml")
	if err := os.WriteFile(p, []byte("users:\n  - name: ro\n    passwordHash: \"$2a$10$abc\"\n    role: ReadOnly\n  - name: rw\n    passwordHash: \"$2a$10$def\"\n    role: ReadWrite\n"), 0600); err != nil {
		t.Fatal(err)
	}
	users, err := LoadUsers(p)
	if err != nil {
		t.Fatal(err)
	}
	if len(users) != 2 || users[0].Name != "ro" || users[1].Role != "ReadWrite" {
		t.Fatalf("parsed users wrong: %+v", users)
	}
}

func TestLoadUsers_MissingFileIsError(t *testing.T) {
	if _, err := LoadUsers(filepath.Join(t.TempDir(), "nope.yaml")); err == nil {
		t.Fatal("expected error for missing file")
	}
}
