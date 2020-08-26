package app

import "os"

var Namespace = "d8-upmeter"
var UpmeterHost = "localhost"
var UpmeterPort = "8091"
var UpmeterListenHost = "0.0.0.0"
var UpmeterListenPort = "8091"
var DowntimeDbPath = "downtime.db.sqlite"
var UpmeterCaPath = ""
var UpmeterTls = "false"

func InitAppEnv() {
	Namespace = StringFromEnv("NAMESPACE", Namespace)
	UpmeterHost = StringFromEnv("UPMETER_SERVICE_HOST", UpmeterHost)
	UpmeterPort = StringFromEnv("UPMETER_SERVICE_PORT", UpmeterPort)
	UpmeterListenHost = StringFromEnv("UPMETER_LISTEN_HOST", UpmeterListenHost)
	UpmeterListenPort = StringFromEnv("UPMETER_LISTEN_PORT", UpmeterListenPort)
	DowntimeDbPath = StringFromEnv("UPMETER_DB_PATH", DowntimeDbPath)
	UpmeterCaPath = StringFromEnv("UPMETER_CA_PATH", UpmeterCaPath)
	UpmeterTls = StringFromEnv("UPMETER_TLS", UpmeterTls)
}

func StringFromEnv(envName string, defValue string) string {
	newVal := os.Getenv(envName)
	if newVal != "" {
		return newVal
	}
	return defValue
}
