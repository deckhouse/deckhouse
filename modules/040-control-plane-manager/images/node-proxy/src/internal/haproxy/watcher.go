package haproxy

import (
	"errors"
	"fmt"

	log "github.com/sirupsen/logrus"

	"node-proxy-sidecar/internal/config"
)

func (c *Client) BackendSync(backend config.Backend, desiredServers []Server) {
	var curBackend string
	cfg, err := c.client.Configuration()
	if err != nil {
		log.Error("unable to read configuration", err)
	}

	c.runtimeClient, err = c.client.Runtime()
	if err != nil {
		log.Error("unable to create runtime client", err)
	}

	_, rawBackends, err := cfg.GetBackends("")
	if err != nil {
		log.Error("unable to read backends from configuration", err)
	}

	for _, b := range rawBackends {
		if b.Name == backend.Name {
			curBackend = b.Name
			break
		}
	}

	if curBackend == "" {
		log.Infof("Backend with name %s not found in Haproxy Config", backend.Name)
		// return err
	}

	currentServers, err := c.getServes(backend.Name)
	if err != nil {
		log.Error(err)
	}

	serversToAdd, serversToRemove := c.diffServers(currentServers, desiredServers)

	if len(serversToAdd) > 0 {
		log.Infoln("serversToAdd", serversToAdd)
	}
	if len(serversToRemove) > 0 {
		log.Infoln("serversToRemove", serversToRemove)
	}

	c.addServer(backend, serversToAdd)
	c.delServer(backend, serversToRemove)
}

func (c *Client) addServer(backend config.Backend, servers []Server) {
	for _, server := range servers {
		serverName := fmt.Sprintf("%s_%d", server.Address, server.Port)
		addr := fmt.Sprintf("%s:%d", server.Address, server.Port)
		attr := fmt.Sprintf("%s %s", addr, backend.HAProxy.DefautlServer)
		err := c.runtimeClient.AddServer(backend.Name, serverName, attr)
		if err != nil {
			log.Error(err)
		}
		err = c.runtimeClient.EnableServer(backend.Name, serverName)
		if err != nil {
			log.Error(err)
		}
		err = c.runtimeClient.EnableServerHealth(backend.Name, serverName)
		if err != nil {
			log.Error(err)
		}
	}
}

func (c *Client) delServer(backend config.Backend, servers []Server) {
	for _, server := range servers {
		err := c.runtimeClient.DisableServer(backend.Name, server.Name)
		if err != nil {
			log.Error(err)
		}
		err = c.runtimeClient.DeleteServer(backend.Name, server.Name)
		if err != nil {
			log.Error(err)
		}
	}
}

func (c *Client) diffServers(current, desired []Server) (toAdd []Server, toRemove []Server) {
	currentMap := make(map[string]Server)
	for _, s := range current {
		key := fmt.Sprintf("%s:%d", s.Address, s.Port)
		currentMap[key] = s
	}

	desiredMap := make(map[string]Server)
	for _, s := range desired {
		key := fmt.Sprintf("%s:%d", s.Address, s.Port)
		desiredMap[key] = s
		if _, exists := currentMap[key]; !exists {
			toAdd = append(toAdd, s)
		}
	}

	for _, s := range current {
		key := fmt.Sprintf("%s:%d", s.Address, s.Port)
		if _, exists := desiredMap[key]; !exists {
			toRemove = append(toRemove, s)
		}
	}

	return
}

func (c *Client) getServes(backend string) ([]Server, error) {
	rawServers, err := c.runtimeClient.GetServersState(backend)
	if err != nil {
		log.Infof("unable to get servers for backed: %s", backend)

		return nil, err
	}

	servers := make([]Server, 0, 1)
	for _, server := range rawServers {
		servers = append(servers, Server{Address: server.Address, Port: *server.Port, Name: server.Name})
	}

	if len(servers) == 0 {
		return nil, errors.New("no servers for backend")
	}

	return servers, nil
}
