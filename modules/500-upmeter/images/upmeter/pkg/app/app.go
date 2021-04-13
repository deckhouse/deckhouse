package app

import (
	"os"
)

var (
	Namespace = "d8-upmeter"

	ServiceHost = "localhost"
	ServicePort = "8091"

	ListenHost = "0.0.0.0"
	ListenPort = "8091"

	DatabasePath           = "downtime.db.sqlite"
	DatabaseMigrationsPath = "."

	CaPath = ""
	Tls    = "false"
)

func InitAppEnv() {
	Namespace = StringFromEnv("NAMESPACE", Namespace)

	ServiceHost = StringFromEnv("UPMETER_SERVICE_HOST", ServiceHost)
	ServicePort = StringFromEnv("UPMETER_SERVICE_PORT", ServicePort)

	ListenHost = StringFromEnv("UPMETER_LISTEN_HOST", ListenHost)
	ListenPort = StringFromEnv("UPMETER_LISTEN_PORT", ListenPort)

	DatabasePath = StringFromEnv("UPMETER_DB_PATH", DatabasePath)
	DatabaseMigrationsPath = StringFromEnv("UPMETER_DB_MIGRATIONS_PATH", DatabaseMigrationsPath)

	CaPath = StringFromEnv("UPMETER_CA_PATH", CaPath)
	Tls = StringFromEnv("UPMETER_TLS", Tls)
}

func StringFromEnv(envName string, defValue string) string {
	newVal := os.Getenv(envName)
	if newVal != "" {
		return newVal
	}
	return defValue
}
