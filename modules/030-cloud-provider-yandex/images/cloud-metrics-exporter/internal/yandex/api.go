// Copyright 2022 Flant JSC
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

package yandex

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
	ycsdk "github.com/yandex-cloud/go-sdk"
	"github.com/yandex-cloud/go-sdk/iamkey"
)

const (
	prometheusMetricsUrl = "https://monitoring.api.cloud.yandex.net/monitoring/v2/prometheusMetrics"
	retries              = 3
)

var services = map[string]struct{}{
	// Compute Cloud
	"compute": {},
	// Object Storage
	"storage": {},
	// Managed Service for PostgreSQL
	"managed-postgresql": {},
	// Managed Service for ClickHouse;
	"managed-clickhouse": {},
	// Managed Service for MongoDB;
	"managed-mongodb": {},
	// Managed Service for MySQL;
	"managed-mysql": {},
	// Managed Service for Redis;
	"managed-redis": {},
	// Managed Service for Apache Kafka®;
	"managed-kafka": {},
	// Managed Service for Elasticsearch;
	"managed-elasticsearch": {},
	// Managed Service for SQL Server
	"managed-sqlserver": {},
	// Managed Service for Kubernetes;
	"managed-kubernetes": {},
	// Cloud Functions
	"serverless-functions": {},
	// Cloud Functions triggers
	"serverless_triggers_client_metrics": {},
	// Yandex Database
	"ydb": {},
	// Cloud Interconnect;
	"interconnect": {},
	// Certificate Manager;
	"certificate-manager": {},
	// Data Transfer
	"data-transfer": {},
	// Data Proc
	"data-proc": {},
	// API Gateway.
	"serverless-apigateway": {},
}

type CloudApi struct {
	folderId        string
	stopCh          chan struct{}
	logger          *log.Entry
	autoRenewPeriod time.Duration
	onRenewError    func()
	client          *http.Client

	tokenMutex sync.RWMutex
	token      string

	isInit bool
	iamKey *iamkey.Key
}

func NewCloudAPI(logger *log.Entry, folderId string, stopCh chan struct{}) *CloudApi {
	client := &http.Client{
		Transport: &http.Transport{
			DialContext: (&net.Dialer{
				Timeout: 30 * time.Second,
			}).DialContext,
		},
	}

	return &CloudApi{
		folderId: folderId,
		logger:   logger,
		// iam token available during 12 hours, but yandex recommend update renew token every one hour
		autoRenewPeriod: 1 * time.Hour,
		stopCh:          stopCh,
		client:          client,
	}
}

func (a *CloudApi) WithAutoRenewPeriod(autoRenewPeriod time.Duration) *CloudApi {
	a.autoRenewPeriod = autoRenewPeriod

	return a
}

func (a *CloudApi) WithRenewTokenErrorHandler(handler func()) *CloudApi {
	a.onRenewError = handler

	return a
}

func (a *CloudApi) HasService(key string) bool {
	_, has := services[key]

	return has
}

func (a *CloudApi) InitWithAPIKey(key string) {
	if a.isInit {
		a.logger.Warningln("Yandex cloud api already init")
		return
	}

	a.setToken(strings.TrimSpace(key))
	a.isInit = true
}

func (a *CloudApi) InitWithServiceAccount(serviceAccount io.Reader) error {
	if a.isInit {
		a.logger.Warningln("Yandex cloud api already init")
		return nil
	}

	var iamKey iamkey.Key
	decoder := json.NewDecoder(serviceAccount)

	err := decoder.Decode(&iamKey)
	if err != nil {
		return errors.Wrap(err, "malformed service account json")
	}

	a.iamKey = &iamKey

	err = a.renewToken()
	if err != nil {
		return err
	}

	go a.startAutoRenewToken()

	a.isInit = true

	return nil
}

func (a *CloudApi) RequestMetrics(ctx context.Context, serviceId string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, a.url(serviceId), nil)
	if err != nil {
		return nil, fmt.Errorf("failed creating request: %s", err)
	}

	token := a.getToken()
	req.Header.Set("Authorization", "Bearer "+token)

	response, err := a.client.Do(req)

	if response != nil && response.Body != nil {
		defer response.Body.Close()
	}

	if e, ok := err.(net.Error); ok && e.Timeout() {
		return nil, fmt.Errorf("do request timeout: %v", err)
	} else if err != nil {
		return nil, fmt.Errorf("failed send request: %s", err)
	}

	if response.StatusCode != http.StatusOK {
		responseData, err := ioutil.ReadAll(response.Body)
		if err != nil {
			errStr := fmt.Errorf("parse error for response body: %s", err).Error()
			responseData = []byte(errStr)
		}

		return nil, fmt.Errorf("status code %v, error response: %s", response.StatusCode, string(responseData))
	}

	return io.ReadAll(response.Body)
}

func (a *CloudApi) url(serviceId string) string {
	u, _ := url.Parse(prometheusMetricsUrl)

	query := u.Query()
	query.Set("service", serviceId)
	query.Set("folderId", a.folderId)

	u.RawQuery = query.Encode()

	return u.String()
}

func (a *CloudApi) getToken() string {
	a.tokenMutex.RLock()
	defer a.tokenMutex.RUnlock()

	return a.token
}

func (a *CloudApi) setToken(token string) {
	a.tokenMutex.Lock()
	defer a.tokenMutex.Unlock()

	a.token = token
}

func (a *CloudApi) startAutoRenewToken() {
	a.logger.Info("Start auto renew IAM-token")
	a.logger.Warn("Stop auto renew IAM-token")

	t := time.NewTicker(a.autoRenewPeriod)
	defer t.Stop()

	for {
		select {
		case <-a.stopCh:
			return
		case <-t.C:
			err := a.renewToken()
			if err != nil {
				a.logger.Errorf("Cannot auto-renew IAM-token: %v", err)
				if a.onRenewError != nil {
					a.onRenewError()
				}
				return
			}
		}
	}
}

func (a *CloudApi) renewToken() error {
	a.logger.Info("Start getting new IAM-token")

	rawCreds, err := ycsdk.ServiceAccountKey(a.iamKey)
	if err != nil {
		return errors.Wrap(err, "invalid auth credentials")
	}

	iamCreds, ok := rawCreds.(ycsdk.ExchangeableCredentials)
	if !ok {
		return fmt.Errorf("cannot convert rawCreds to ExchangeableCredentials")
	}

	request, err := iamCreds.IAMTokenRequest()
	if err != nil {
		return fmt.Errorf("cannot get IAMToken request: %v", err)
	}

	jwtForExchange := request.GetJwt()

	token := ""
	var lastErr error

	for i := 1; i <= retries; i++ {
		token, lastErr = a.requestToken(jwtForExchange)
		if lastErr == nil {
			break
		}
	}

	if token == "" {
		return fmt.Errorf("cannot get IAM-token after %d retries, last error: %v", retries, lastErr)
	}

	a.setToken(token)

	a.logger.Info("Getting new IAM-token was successfully")

	return nil
}

func (a *CloudApi) requestToken(jwtForExchange string) (string, error) {
	body := strings.NewReader(fmt.Sprintf(`{"jwt":"%s"}`, jwtForExchange))
	req, err := http.NewRequest("POST", "https://iam.api.cloud.yandex.net/iam/v1/tokens", body)
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := a.client.Do(req)
	if err != nil {
		return "", err
	}

	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := ioutil.ReadAll(resp.Body)
		panic(fmt.Sprintf("%s: %s", resp.Status, body))
	}

	var data struct {
		IAMToken string `json:"iamToken"`
	}

	err = json.NewDecoder(resp.Body).Decode(&data)
	if err != nil {
		return "", err
	}

	return data.IAMToken, err
}
