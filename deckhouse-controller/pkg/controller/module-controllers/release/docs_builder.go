package release

import (
	"fmt"
	"io"
	"net/http"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	v1 "github.com/google/go-containerregistry/pkg/v1"

	d8http "github.com/deckhouse/deckhouse/go_lib/dependency/http"
	"github.com/deckhouse/deckhouse/go_lib/module"
)

func buildDocumentation(client d8http.Client, img v1.Image, moduleName, moduleVersion string, values *go_hook.PatchableValues) error {
	if !isModuleEnabled("documentation", values) {
		return nil
	}

	rc := module.ExtractDocs(img)
	defer rc.Close()

	const docsBuilderBasePath = "http://documentation-builder.d8-system.svc.cluster.local:8081"

	url := fmt.Sprintf("%s/loadDocArchive/%s/%s", docsBuilderBasePath, moduleName, moduleVersion)
	response, statusCode, err := httpPost(client, url, rc)
	if err != nil {
		return fmt.Errorf("POST %q return %d %q: %w", url, statusCode, response, err)
	}

	url = fmt.Sprintf("%s/build", docsBuilderBasePath)
	response, statusCode, err = httpPost(client, url, nil)
	if err != nil {
		return fmt.Errorf("POST %q return %d %q: %w", url, statusCode, response, err)
	}

	return nil
}

func httpPost(httpClient d8http.Client, url string, body io.Reader) ([]byte, int, error) {
	req, err := http.NewRequest(http.MethodPost, url, body)
	if err != nil {
		return nil, 0, err
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

func isModuleEnabled(moduleName string, values *go_hook.PatchableValues) (enabled bool) {
	for _, mod := range values.Get("global.enabledModules").Array() {
		if mod.String() == moduleName {
			return true
		}
	}

	return false
}
