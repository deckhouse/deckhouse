/*
Copyright 2021 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package internal

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"regexp"
	"strings"

	"github.com/flant/shell-operator/pkg/kube_events_manager/types"

	d8http "github.com/deckhouse/deckhouse/go_lib/dependency/http"
)

const namespace = "d8-istio"

func Queue(name string) string {
	return fmt.Sprintf("/modules/istio/%s", name)
}

func NsSelector() *types.NamespaceSelector {
	return &types.NamespaceSelector{
		NameSelector: &types.NameSelector{
			MatchNames: []string{namespace},
		},
	}
}

func Contains(s []string, str string) bool {
	for _, v := range s {
		if v == str {
			return true
		}
	}

	return false
}

func HTTPGet(httpClient d8http.Client, url string, bearerToken string) ([]byte, int, error) {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, 0, err
	}

	if len(bearerToken) > 0 {
		req.Header.Add("Authorization", "Bearer "+bearerToken)
	}

	res, err := httpClient.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer res.Body.Close()

	dataBytes, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return nil, 0, err
	}

	return dataBytes, res.StatusCode, nil
}

func VersionToRevision(version string) string {
	// Restore 'v' prefix.
	if !strings.HasPrefix(version, "v") {
		version = "v" + version
	}

	// Check if version is already converted.
	if !strings.ContainsAny(version, ".-") {
		return version
	}

	// v1.2.3-alpha.4 -> v1.2.3-alpha4
	var re = regexp.MustCompile(`([a-z])\.([0-9])`)
	version = re.ReplaceAllString(version, `$1$2`)

	// v1.2.3-alpha4 -> v1x2x3-alpha4
	version = strings.ReplaceAll(version, ".", "x")

	// v1x2x3-alpha4 -> v1x2x3alpha4
	version = strings.ReplaceAll(version, "-", "")

	return version
}
