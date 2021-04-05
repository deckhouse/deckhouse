package sender

import (
	"bytes"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io/ioutil"
	"net/http"
	"time"

	log "github.com/sirupsen/logrus"

	"upmeter/pkg/app"
)

type UpmeterClient struct {
	url    string
	client *http.Client
}

func NewUpmeterClient(ip, port string, timeout time.Duration) *UpmeterClient {
	schema := "https"
	if app.UpmeterTls == "false" {
		schema = "http"
	}

	url := fmt.Sprintf("%s://%s:%s/downtime", schema, ip, port)

	return &UpmeterClient{
		url:    url,
		client: NewHttpClient(timeout),
	}
}

func (c *UpmeterClient) Send(reqBody []byte) error {
	req, err := http.NewRequest(http.MethodPost, c.url, bytes.NewReader(reqBody))
	if err != nil {
		return fmt.Errorf("cannot create POST request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("did not send to upmeter: %v", err)
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("cannot read upmeter response body: %v", err)
	}

	if resp.StatusCode != 200 {
		return fmt.Errorf("unexpected upmeter response status=%d, body=%q", resp.StatusCode, string(body))
	}

	return nil
}

func NewHttpClient(timeout time.Duration) *http.Client {
	client, err := createSecureHttpClient(timeout)
	if err != nil {
		log.Errorf("falling back to default HTTP client: %v", err)
		return &http.Client{Timeout: timeout}
	}
	return client
}

func createSecureHttpClient(timeout time.Duration) (*http.Client, error) {
	if app.UpmeterTls == "false" {
		return nil, fmt.Errorf("TLS is off by client")
	}

	tlsTransport, err := createHttpTransport()
	if err != nil {
		return nil, err
	}

	// Wrap tls transport to add Authorization header.
	bearerToken, err := getServiceAccountToken()
	if err != nil {
		return nil, err
	}

	// Create https client with checking CA certificate and Authorization header
	client := &http.Client{
		Transport: NewKubeBearerTransport(tlsTransport, bearerToken),
		Timeout:   timeout,
	}

	return client, nil
}

func createHttpTransport() (*http.Transport, error) {
	if app.UpmeterCaPath == "" {
		tr := &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true,
			},
		}
		return tr, nil
	}

	// Create transport with tls and CA certificate checking
	caCertBytes, err := ioutil.ReadFile(app.UpmeterCaPath)
	if err != nil {
		return nil, fmt.Errorf("cannot read CA certificate from '%s': %v", app.UpmeterCaPath, err)
	}

	caCertPool := x509.NewCertPool()
	caCertPool.AppendCertsFromPEM(caCertBytes)

	tr := &http.Transport{
		TLSClientConfig: &tls.Config{
			RootCAs: caCertPool,
		},
	}

	return tr, nil
}

func getServiceAccountToken() (string, error) {
	bs, err := ioutil.ReadFile("/var/run/secrets/kubernetes.io/serviceaccount/token")
	if err != nil {
		return "", fmt.Errorf("cannot read service account file: %v", err)
	}
	return string(bs), nil
}

func NewKubeBearerTransport(next http.RoundTripper, bearer string) *KubeBearerTransport {
	return &KubeBearerTransport{
		next:        next,
		bearerToken: bearer,
	}
}

type KubeBearerTransport struct {
	next        http.RoundTripper
	bearerToken string
}

func (t *KubeBearerTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	req.Header.Add("Authorization", "Bearer "+t.bearerToken)
	return t.next.RoundTrip(req)
}
