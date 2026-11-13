// Copyright 2026 Flant JSC
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

package s3cache

import (
	"net/http"
	"strings"
	"testing"
	"time"
)

// The examples of the AWS documentation "Signature calculations for the
// Authorization header: transferring payload in a single chunk".
func TestSignAWSExamples(t *testing.T) {
	const (
		accessKey = "AKIAIOSFODNN7EXAMPLE"
		secretKey = "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY"
	)
	now := time.Date(2013, 5, 24, 0, 0, 0, 0, time.UTC)

	for _, tt := range []struct {
		name, url, signature string
		headers              map[string]string
	}{
		{
			name:      "GET object",
			url:       "https://examplebucket.s3.amazonaws.com/test.txt",
			headers:   map[string]string{"Range": "bytes=0-9"},
			signature: "f0e8bdb87c964420e857bd35b5d6ed310bd44f0170aba48dd91039c6036bdb41",
		},
		{
			name:      "GET bucket (list objects)",
			url:       "https://examplebucket.s3.amazonaws.com/?max-keys=2&prefix=J",
			signature: "34b48302e7b5fa45bde8084f4b7868a86f0a534bc59db6670ed5711ef69dc6f7",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			req, err := http.NewRequest(http.MethodGet, tt.url, nil)
			if err != nil {
				t.Fatal(err)
			}
			for k, v := range tt.headers {
				req.Header.Set(k, v)
			}

			sign(req, accessKey, secretKey, "", "us-east-1", emptyPayload, now)

			auth := req.Header.Get("Authorization")
			if !strings.HasSuffix(auth, "Signature="+tt.signature) {
				t.Fatalf("Authorization = %s, want signature %s", auth, tt.signature)
			}
		})
	}
}
