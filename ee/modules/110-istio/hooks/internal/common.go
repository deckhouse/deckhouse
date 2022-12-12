/*
Copyright 2021 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package internal

import (
	"fmt"
	"io"
	"net/http"

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

	dataBytes, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, 0, err
	}

	return dataBytes, res.StatusCode, nil
}
