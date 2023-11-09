package sender

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"registry-modules-watcher/internal/backends"
	"strings"
	"time"

	"github.com/hashicorp/go-retryablehttp"
	"k8s.io/klog"
)

type sender struct {
	client *http.Client
}

// New
func New() *sender {
	retryClient := retryablehttp.NewClient()
	retryClient.RetryMax = 5

	client := retryClient.StandardClient()
	client.Timeout = 30 * time.Second

	return &sender{
		client: client,
	}
}

func (s *sender) Send(ctx context.Context, listBackends map[string]struct{}, versions []backends.Version) error {
	syncChan := make(chan struct{}, 10)
	for backend := range listBackends {
		syncChan <- struct{}{}

		go func(backend string) {
			for _, version := range versions {
				url := "http://" + backend + "/loadDocArchive/" + version.Module + "/" + version.Version + "?channels=" + strings.Join(version.ReleaseChannels, ",")
				err := s.loadDocArchive(ctx, url, version.TarFile)
				if err != nil {
					klog.Errorf("send docs error: %v", err)
				}
			}

			url := "http://" + backend + "/build"
			err := s.build(ctx, url)
			if err != nil {
				klog.Errorf("build docs error: %v", err)
			}
			<-syncChan
		}(backend)
	}

	return nil
}

func (s *sender) loadDocArchive(ctx context.Context, url string, tarFile []byte) error {
	klog.V(2).Infof("send loadDoc url: %s", url)
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(tarFile))
	if err != nil {
		return fmt.Errorf("client: could not create request: %s", err)
	}

	req.Header.Set("Content-Type", "application/tar")

	resp, err := s.client.Do(req)
	if err != nil {
		return fmt.Errorf("client: error making http request: %s", err)
	}

	klog.V(2).Infof("SendTars resp: %s", resp.Status)
	return nil
}

func (s *sender) build(ctx context.Context, url string) error {
	klog.V(2).Infof("send build url: %s", url)
	req, err := http.NewRequest(http.MethodPost, url, nil)
	if err != nil {
		return fmt.Errorf("client: could not create request: %s", err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := s.client.Do(req)
	if err != nil {
		return fmt.Errorf("client: error making http request: %s", err)
	}

	klog.V(2).Info("SendBuild resp: ", resp.StatusCode)

	return nil
}
