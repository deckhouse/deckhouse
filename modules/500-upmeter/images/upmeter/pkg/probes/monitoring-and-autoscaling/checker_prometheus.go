package monitoring_and_autoscaling

import (
	"fmt"
	"io/ioutil"
	"net/http"

	"github.com/tidwall/gjson"

	"upmeter/pkg/checks"
)

// commonPodChecker implements boilerplate and is intended to be embedded into other pod checker implementations
type commonPodChecker struct {
	namespace     string
	labelSelector string
}

func (c commonPodChecker) Namespace() string {
	return c.namespace
}

func (c commonPodChecker) LabelSelector() string {
	return c.labelSelector
}

// promChecker implements the check for prometheus and trickster
type promChecker struct {
	commonPodChecker

	client *http.Client

	namespace     string
	labelSelector string
	endpoint      string
}

func (pc promChecker) Endpoint() string {
	return pc.endpoint
}

/*
	Expecting JSON like this

	{
	  "status": "success",
	  "data": {
	    "resultType": "vector",
	    "result": [
	      {
	        "metric": {},
	        "value": [
	          1613143228.991,
	          "1"               <- we check this
	        ]
	      }
	    ]
	  }
	}
*/
func (pc promChecker) Verify(body []byte) checks.Error {
	path := "data.result.0.value.1"
	value := gjson.Get(string(body), path)

	if value.String() != "1" {
		return checks.ErrFail(`cannot find "1" in path %q prometheus data %q`, path, body)
	}

	return nil
}

// newRequest prepares request object for given URL with auth token
func newRequest(endpoint, authToken string) (*http.Request, checks.Error) {
	req, err := http.NewRequest("GET", endpoint, nil)
	if err != nil {
		return nil, checks.ErrUnknownResult("did not create request: %s", err)
	}
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", authToken))

	return req, nil
}

// doRequest handles
func doRequest(client *http.Client, req *http.Request) ([]byte, checks.Error) {
	resp, err := client.Do(req)
	if err != nil {
		return nil, checks.ErrUnknownResult(`cannot dial %q: %v`, req.URL, err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, checks.ErrFail(`got HTTP status %q`, resp.Status)
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, checks.ErrFail("cannot read response body: %v", err)
	}
	defer resp.Body.Close()

	return body, nil
}
