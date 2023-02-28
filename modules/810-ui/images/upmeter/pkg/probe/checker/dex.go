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

package checker

import (
	"net/http"
	"time"

	"github.com/tidwall/gjson"

	"d8.io/upmeter/pkg/check"
	"d8.io/upmeter/pkg/kubernetes"
)

// DexApiAvailable is a checker constructor and configurator
type DexApiAvailable struct {
	Access   kubernetes.Access
	Timeout  time.Duration
	Endpoint string
}

func (c DexApiAvailable) Checker() check.Checker {
	verifier := dexAPIVerifier{
		endpoint: c.Endpoint,
		access:   c.Access,
	}
	checker := newHTTPChecker(newInsecureClient(3*c.Timeout), verifier)
	return withTimeout(checker, c.Timeout)
}

type dexAPIVerifier struct {
	endpoint string
	access   kubernetes.Access
}

func (v dexAPIVerifier) Request() *http.Request {
	req, err := newGetRequest(v.endpoint, v.access.ServiceAccountToken(), v.access.UserAgent())
	if err != nil {
		panic(err)
	}
	return req
}

/*
Expected JSON

	{
	  "keys": [
		{
		  "use": "sig",
		  "kty": "RSA",
		  "kid": "a10f8.....87",
		  "alg": "RS256",
		  "n": "vBe2Na.............dSHBw",
		  "e": "AQAB"
		},
		{
		  "use": "sig",
		  "kty": "RSA",
		  "kid": "d816e.....bd49",
		  "alg": "RS256",
		  "n": "qgUD1y............ufPLQ",
		  "e": "AQAB"
		}
	  ]
	}
*/
func (v dexAPIVerifier) Verify(body []byte) check.Error {
	path := "keys"
	value := gjson.Get(string(body), path)

	if !value.IsArray() {
		return check.ErrFail(`cannot find array in path %q in dex response %q`, path, body)
	}

	return nil
}
