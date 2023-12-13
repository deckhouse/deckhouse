package module

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"time"

	d8http "github.com/deckhouse/deckhouse/go_lib/dependency/http"
)

func NewDocsBuilderClient(httpClient d8http.Client) *DocsBuilderClient {
	return &DocsBuilderClient{httpClient: httpClient}
}

type DocsBuilderClient struct {
	httpClient d8http.Client
}

func (c *DocsBuilderClient) SendDocumentation(baseAddr string, moduleName, moduleVersion string, docsArchive io.Reader) error {
	url := fmt.Sprintf("%s/loadDocArchive/%s/%s", baseAddr, moduleName, moduleVersion)
	response, statusCode, err := c.httpPost(url, docsArchive)
	if err != nil {
		return fmt.Errorf("POST %q: %w", url, err)
	}

	if statusCode != http.StatusCreated {
		return fmt.Errorf("POST %q: [%d] %q", url, statusCode, response)
	}

	return nil
}

func (c *DocsBuilderClient) BuildDocumentation(docsBuilderBasePath string) error {
	url := fmt.Sprintf("%s/build", docsBuilderBasePath)
	response, statusCode, err := c.httpPost(url, nil)
	if err != nil {
		return fmt.Errorf("POST %q: %w", url, err)
	}

	if statusCode != http.StatusOK {
		return fmt.Errorf("POST %q: [%d] %q", url, statusCode, response)
	}

	return nil
}

func (c *DocsBuilderClient) httpPost(url string, body io.Reader) ([]byte, int, error) {
	req, err := http.NewRequest(http.MethodPost, url, body)
	if err != nil {
		return nil, 0, err
	}

	res, err := c.httpClient.Do(req)
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

func (c *DocsBuilderClient) CheckBuilderHealth(ctx context.Context, baseAddr string) error {
	ctx, cancel := context.WithTimeout(ctx, time.Second)
	defer cancel()

	url := fmt.Sprintf("%s/healthz", baseAddr)
	response, statusCode, err := c.httpGet(ctx, url)
	if err != nil {
		return fmt.Errorf("GET %q: %w", url, err)
	}

	if statusCode != http.StatusOK {
		return fmt.Errorf("GET %q: [%d] %q", url, statusCode, response)
	}

	return nil
}

func (c *DocsBuilderClient) httpGet(ctx context.Context, url string) ([]byte, int, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, http.NoBody)
	if err != nil {
		return nil, 0, err
	}

	res, err := c.httpClient.Do(req)
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
