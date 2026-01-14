package controller

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"k8s.io/client-go/rest"
)

type ApiServerVersionGetter interface {
	Get(ctx context.Context, host string) (string, error)
}

type inClusterVersionGetter struct {
	client *http.Client
}

func newInClusterVersionGetter(cfg *rest.Config) (*inClusterVersionGetter, error) {
	transport, err := rest.TransportFor(cfg)
	if err != nil {
		return nil, err
	}

	return &inClusterVersionGetter{
		client: &http.Client{
			Transport: transport,
			Timeout:   5 * time.Second,
		},
	}, nil
}

func (vg *inClusterVersionGetter) Get(ctx context.Context, host string) (string, error) {
	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodGet,
		fmt.Sprintf("https://%s:6443/version", host),
		nil,
	)
	if err != nil {
		return "", err
	}

	resp, err := vg.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("unexpected status: %s", resp.Status)
	}

	var v struct {
		GitVersion string `json:"gitVersion"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&v); err != nil {
		return "", err
	}

	return v.GitVersion, nil
}

type fakeVersionGetter struct {
	versions map[string]string
	errors   map[string]error
}

func (f *fakeVersionGetter) Get(
	_ context.Context,
	host string,
) (string, error) {
	if err, ok := f.errors[host]; ok {
		return "", err
	}
	return f.versions[host], nil
}
