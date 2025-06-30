/*
Copyright 2025 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package main

import "os"

type Config struct {
	ServiceName string
	Namespace   string
	SA          string
}

func LoadConfig() Config {
	return Config{
		ServiceName: getEnvOrDefault("SERVICE_NAME_ISTIOD", "istiod"),
		Namespace:   getEnvOrDefault("NAMESPACE_ISTIOD", "d8-istio"),
		SA: getEnvOrDefault("SA_MONITOR", "multicluster-metrics-exporter"),
	}
}

func getEnvOrDefault(key, def string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return def
}
