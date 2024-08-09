/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package main

import (
	"fmt"
	"net/http"
	"os"
	"strings"
	"syscall"
	"time"

	"gopkg.in/yaml.v2"
)

const (
	checkInterval      = 5 * time.Second
	checkTimeout       = 5 * time.Second
	maxConnectionTries = 30
	appPath            = "/registry"
)

type Config struct {
	Storage struct {
		S3 struct {
			RegionEndpoint string `yaml:"regionendpoint"`
		} `yaml:"s3"`
	} `yaml:"storage"`
}

func readConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

func checkEndpoint(url string) bool {
	client := http.Client{Timeout: checkTimeout}
	for i := 0; i < maxConnectionTries; i++ {
		if available, _ := attemptConnection(client, url); available {
			return true
		}
		fmt.Printf("Endpoint %s is not available, attempt %d/%d, retrying in %d seconds\n", url, i+1, maxConnectionTries, checkInterval/time.Second)
		time.Sleep(checkInterval)
	}
	return false
}

func attemptConnection(client http.Client, url string) (bool, error) {
	resp, err := client.Get(url)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()
	return resp.StatusCode >= 200 && resp.StatusCode < 500, nil
}

func main() {
	configPath := ""
	endpoint := ""

	for _, arg := range os.Args {
		if strings.HasSuffix(arg, ".yaml") {
			configPath = arg
			break
		}
	}

	if configPath == "" {
		fmt.Println("No YAML configuration file specified in the command line arguments. " +
			"The S3 endpoint check will be skipped.")
	} else {
		config, err := readConfig(configPath)
		switch {
		case err != nil:
			fmt.Printf("Error reading the configuration file: %v. The S3 endpoint check will be skipped.\n", err)
		case config.Storage.S3.RegionEndpoint == "":
			fmt.Println("The 'regionendpoint' is not specified in the configuration file. The S3 endpoint check will be skipped.")
		default:
			endpoint = config.Storage.S3.RegionEndpoint
		}
	}

	if endpoint != "" {
		fmt.Printf("Checking S3 endpoint %s...\n", endpoint)
		if !checkEndpoint(endpoint) {
			fmt.Printf("Unable to connect to the specified S3 endpoint at %s after %d attempts.", endpoint, maxConnectionTries)
			return
		}
	}

	fmt.Printf("Starting Docker Distribution: %s %s\n", appPath, os.Args)
	env := os.Environ()
	if err := syscall.Exec(appPath, os.Args, env); err != nil {
		fmt.Printf("Failed to execute Docker Distribution due to the following error: %v\n", err)
		return
	}
}
