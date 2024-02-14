/*
Copyright 2021 Flant JSC

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

// Copied from [1] to control timeseries timestamps manually
//      [1] go.opentelemetry.io/contrib/exporters/metric/cortex@v0.17.0/

package remotewrite

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/gogo/protobuf/proto"
	"github.com/golang/snappy"
	"github.com/prometheus/prometheus/prompb"
	"go.opentelemetry.io/contrib/exporters/metric/cortex"
)

// exporter forwards metrics to a remote_write storage
type exporter struct {
	config cortex.Config
}

// Export sends metrics via HTTP
func (e *exporter) Export(ctx context.Context, timeseries []*prompb.TimeSeries) error {
	message, buildMessageErr := e.buildMessage(timeseries)
	if buildMessageErr != nil {
		return buildMessageErr
	}

	request, buildRequestErr := e.buildRequest(ctx, message)
	if buildRequestErr != nil {
		return buildRequestErr
	}

	sendRequestErr := e.sendRequest(request)
	if sendRequestErr != nil {
		return sendRequestErr
	}

	return nil
}

// sendRequest sends an http request using the exporter's http Client.
func (e *exporter) sendRequest(req *http.Request) error {
	// Set a client if the user didn't provide one.
	if e.config.Client == nil {
		client, err := e.buildClient()
		if err != nil {
			return err
		}
		e.config.Client = client
	}

	// Attempt to send request.
	res, err := e.config.Client.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	return e.mapResponseToError(res)
}

var (
	ErrInternalStorageError = fmt.Errorf("storage internal error")
	ErrNotAcceptedByStorage = fmt.Errorf("not accepted by storage")
	ErrNoCompleteEpisodes   = fmt.Errorf("no complete episodes for export")
)

func (e *exporter) mapResponseToError(res *http.Response) error {
	if res.StatusCode >= 500 {
		// should retry, the storage will recover eventually
		return errWithStatusAndBody(res, ErrInternalStorageError)
	}

	if res.StatusCode >= 400 {
		// storage did not accept the data, re-sending will not help
		return errWithStatusAndBody(res, ErrNotAcceptedByStorage)
	}

	return nil
}

func errWithStatusAndBody(res *http.Response, exportErr error) error {
	body, err := io.ReadAll(res.Body)
	if err != nil {
		return fmt.Errorf("got %d, %w: (failed to read response body: %s)", res.StatusCode, exportErr, err.Error())
	}
	return fmt.Errorf("got %d, %w: %q", res.StatusCode, exportErr, body)
}

// addHeaders adds required headers, an Authorization header, and all headers in the
// Config Headers map to a http request.
func (e *exporter) addHeaders(req *http.Request) error {
	// Cortex expects Snappy-compressed protobuf messages. These three headers are
	// hard-coded as they should be on every request.
	req.Header.Add("X-Prometheus-Remote-Write-Version", "0.1.0")
	req.Header.Add("Content-Encoding", "snappy")
	req.Header.Set("Content-Type", "application/x-protobuf")

	// Add all user-supplied headers to the request.
	for name, field := range e.config.Headers {
		req.Header.Add(name, field)
	}

	// Add Authorization header if it wasn't already set.
	if _, exists := e.config.Headers["Authorization"]; !exists {
		if err := e.addBearerTokenAuth(req); err != nil {
			return err
		}
		if err := e.addBasicAuth(req); err != nil {
			return err
		}
	}

	return nil
}

// buildMessage creates a Snappy-compressed protobuf message from a slice of TimeSeries.
func (e *exporter) buildMessage(timeseries []*prompb.TimeSeries) ([]byte, error) {
	// Wrap the TimeSeries as a WriteRequest since Cortex requires it.
	writeRequest := &prompb.WriteRequest{
		Timeseries: timeseries,
	}

	// Convert the struct to a slice of bytes and then compress it.
	message, err := proto.Marshal(writeRequest)
	if err != nil {
		return nil, err
	}
	compressed := snappy.Encode(nil, message)

	return compressed, nil
}

// buildRequest creates an http POST request with a Snappy-compressed protocol buffer
// message as the body and with all the headers attached.
func (e *exporter) buildRequest(ctx context.Context, message []byte) (*http.Request, error) {
	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		e.config.Endpoint,
		bytes.NewBuffer(message),
	)
	if err != nil {
		return nil, err
	}

	// Add the required headers and the headers from Config.Headers.
	err = e.addHeaders(req)
	if err != nil {
		return nil, err
	}

	return req, nil
}

// ErrFailedToReadFile occurs when a password / bearer token file exists, but could
// not be read.
var ErrFailedToReadFile = fmt.Errorf("failed to read password / bearer token file")

// addBasicAuth sets the Authorization header for basic authentication using a username
// and a password / password file. The header value is not changed if an Authorization
// header already exists and no action is taken if the exporter is not configured with
// basic authorization credentials.
func (e *exporter) addBasicAuth(req *http.Request) error {
	// No need to add basic auth if it isn't provided or if the Authorization header is
	// already set.
	if _, exists := e.config.Headers["Authorization"]; exists {
		return nil
	}
	if e.config.BasicAuth == nil {
		return nil
	}

	username := e.config.BasicAuth["username"]

	// Use password from password file if it exists.
	passwordFile := e.config.BasicAuth["password_file"]
	if passwordFile != "" {
		file, err := ioutil.ReadFile(passwordFile)
		if err != nil {
			return ErrFailedToReadFile
		}
		password := string(file)
		req.SetBasicAuth(username, password)
		return nil
	}

	// Use provided password.
	password := e.config.BasicAuth["password"]
	req.SetBasicAuth(username, password)

	return nil
}

// addBearerTokenAuth sets the Authorization header for bearer tokens using a bearer token
// string or a bearer token file. The header value is not changed if an Authorization
// header already exists and no action is taken if the exporter is not configured with
// bearer token credentials.
func (e *exporter) addBearerTokenAuth(req *http.Request) error {
	// No need to add bearer token auth if the Authorization header is already set.
	if _, exists := e.config.Headers["Authorization"]; exists {
		return nil
	}

	// Use bearer token from bearer token file if it exists.
	if e.config.BearerTokenFile != "" {
		file, err := ioutil.ReadFile(e.config.BearerTokenFile)
		if err != nil {
			return ErrFailedToReadFile
		}
		bearerTokenString := "Bearer " + string(file)
		req.Header.Set("Authorization", bearerTokenString)
		return nil
	}

	// Otherwise, use bearer token field.
	if e.config.BearerToken != "" {
		bearerTokenString := "Bearer " + e.config.BearerToken
		req.Header.Set("Authorization", bearerTokenString)
	}

	return nil
}

// buildClient returns a http client that uses TLS and has the user-specified proxy and
// timeout.
func (e *exporter) buildClient() (*http.Client, error) {
	// Create a TLS Config struct for use in a custom HTTP Transport.
	tlsConfig, err := e.buildTLSConfig()
	if err != nil {
		return nil, err
	}

	// Create a custom HTTP Transport for the client. This is the same as
	// http.DefaultTransport other than the TLSClientConfig.
	transport := &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: (&net.Dialer{
			Timeout:   30 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          100,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
		TLSClientConfig:       tlsConfig,
	}

	// Convert proxy url to proxy function for use in the created Transport.
	if e.config.ProxyURL != nil {
		proxy := http.ProxyURL(e.config.ProxyURL)
		transport.Proxy = proxy
	}

	client := http.Client{
		Transport: transport,
		Timeout:   e.config.RemoteTimeout,
	}
	return &client, nil
}

// buildTLSConfig creates a new TLS Config struct with the properties from the exporter's
// Config struct.
func (e *exporter) buildTLSConfig() (*tls.Config, error) {
	tlsConfig := &tls.Config{}
	if e.config.TLSConfig == nil {
		return tlsConfig, nil
	}

	// Set the server name if it exists.
	if e.config.TLSConfig["server_name"] != "" {
		tlsConfig.ServerName = e.config.TLSConfig["server_name"]
	}

	// Set InsecureSkipVerify. Viper reads the bool as a string since it is in a map.
	if isv, ok := e.config.TLSConfig["insecure_skip_verify"]; ok {
		var err error
		if tlsConfig.InsecureSkipVerify, err = strconv.ParseBool(isv); err != nil {
			return nil, err
		}
	}

	// Load certificates from CA file if it exists.
	caFile := e.config.TLSConfig["ca_file"]
	if caFile != "" {
		caFileData, err := os.ReadFile(caFile)
		if err != nil {
			return nil, err
		}
		certPool := x509.NewCertPool()
		certPool.AppendCertsFromPEM(caFileData)
		tlsConfig.RootCAs = certPool
	}

	// Load certificates from CA field if it exists.
	ca := e.config.TLSConfig["ca"]
	if ca != "" {
		var certPool *x509.CertPool
		if tlsConfig.RootCAs != nil {
			certPool = tlsConfig.RootCAs
		} else {
			certPool = x509.NewCertPool()
		}
		certPool.AppendCertsFromPEM([]byte(ca))
		tlsConfig.RootCAs = certPool
	}

	// Load the client certificate if it exists.
	certFile := e.config.TLSConfig["cert_file"]
	keyFile := e.config.TLSConfig["key_file"]
	if certFile != "" && keyFile != "" {
		cert, err := tls.LoadX509KeyPair(certFile, keyFile)
		if err != nil {
			return nil, err
		}
		tlsConfig.Certificates = []tls.Certificate{cert}
	}

	return tlsConfig, nil
}
