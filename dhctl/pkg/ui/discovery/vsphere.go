package discovery

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/vmware/govmomi"
	"github.com/vmware/govmomi/session"
	"github.com/vmware/govmomi/vim25"
	"github.com/vmware/govmomi/vim25/soap"
)

type Discoverer struct {
	client *govmomi.Client
}

func NewDiscoverer(providerData map[string]interface{}) (*Discoverer, error) {
	host, ok := providerData["server"].(string)
	if !ok {
		return nil, fmt.Errorf("server property must be a string")
	}

	username, ok := providerData["username"].(string)
	if !ok {
		return nil, fmt.Errorf("username property must be a string")
	}

	password, ok := providerData["password"].(string)
	if !ok {
		return nil, fmt.Errorf("password property must be a string")
	}

	insecure, ok := providerData["insecure"].(bool)
	if !ok {
		return nil, fmt.Errorf("insecure property must be a boolean")
	}

	parsedURL, err := url.Parse(fmt.Sprintf("https://%s:%s@%s/sdk", url.PathEscape(strings.TrimSpace(username)), url.PathEscape(strings.TrimSpace(password)), url.PathEscape(strings.TrimSpace(host))))
	if err != nil {
		return nil, err
	}

	soapClient := soap.NewClient(parsedURL, insecure)
	vimClient, err := vim25.NewClient(context.TODO(), soapClient)
	if err != nil {
		return nil, err
	}

	if !vimClient.IsVC() {
		return nil, fmt.Errorf("Created client not connected to vCenter")
	}

	// vSphere connection is timed out after 30 minutes of inactivity.
	vimClient.RoundTripper = session.KeepAlive(vimClient.RoundTripper, 10*time.Minute)
	govmomiClient := &govmomi.Client{
		Client:         vimClient,
		SessionManager: session.NewManager(vimClient),
	}

	err = govmomiClient.SessionManager.Login(context.TODO(), parsedURL.User)
	if err != nil {
		return nil, fmt.Errorf("Failed to login with provided credentials: %v", err)
	}

	return &Discoverer{
		client: govmomiClient,
	}, nil
}

func (d *Discoverer) GetVMTemplates() ([]string, error) {
	return nil, nil
}
