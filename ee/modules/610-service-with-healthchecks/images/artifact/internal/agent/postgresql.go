/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package agent

import (
	"database/sql"
	"fmt"
	"strings"

	_ "github.com/lib/pq"
)

type PostgreSQLProbeTarget struct {
	targetPort       int
	successThreshold int32
	failureThreshold int32
	successCount     int32
	failureCount     int32
	timeoutSeconds   int32
	dbName           string
	targetHost       string
	query            string
	tlsMode          string
	user             string
	password         string
	clientCert       string
	clientKey        string
	caCert           string
}

func (pt PostgreSQLProbeTarget) GetPort() int {
	return pt.targetPort
}

func (pt PostgreSQLProbeTarget) GetMode() string {
	return "postgresql"
}

func (pt PostgreSQLProbeTarget) SuccessThreshold() int32 {
	return pt.successThreshold
}

func (pt PostgreSQLProbeTarget) FailureThreshold() int32 {
	return pt.failureThreshold
}

func (pt PostgreSQLProbeTarget) SuccessCount() int32 {
	return pt.successCount
}

func (pt PostgreSQLProbeTarget) FailureCount() int32 {
	return pt.failureCount
}

func (pt PostgreSQLProbeTarget) SetSuccessCount(count int32) Prober {
	pt.successCount = count
	return pt
}

func (pt PostgreSQLProbeTarget) SetFailureCount(count int32) Prober {
	pt.failureCount = count
	return pt
}

func (pt PostgreSQLProbeTarget) GetID() string {
	var sb strings.Builder
	sb.WriteString("postgresql#")
	sb.WriteString(pt.targetHost)
	sb.WriteString("#")
	sb.WriteString(fmt.Sprintf("%d", pt.targetPort))
	sb.WriteString("#")
	sb.WriteString(fmt.Sprintf("%s", pt.dbName))
	return sb.String()
}

func (pt PostgreSQLProbeTarget) getConnectionString() string {
	var sb strings.Builder
	options := map[string]string{
		"sslinline":       "true",
		"connect_timeout": fmt.Sprintf("%d", pt.timeoutSeconds),
		"sslmode":         pt.tlsMode,
		"host":            pt.targetHost,
		"port":            fmt.Sprintf("%d", pt.targetPort),
		"user":            pt.user,
		"password":        pt.password,
		"sslcert":         fmt.Sprintf("'%s'", pt.clientCert),
		"sslkey":          fmt.Sprintf("'%s'", pt.clientKey),
		"sslrootcert":     fmt.Sprintf("'%s'", pt.caCert),
		"dbname":          pt.dbName,
	}
	for key, value := range options {
		if value != "" && value != "''" {
			sb.WriteString(key)
			sb.WriteString("=")
			sb.WriteString(value)
			sb.WriteString(" ")
		}
	}
	return sb.String()
}

func (pt PostgreSQLProbeTarget) PerformCheck() error {
	connString := pt.getConnectionString()
	conn, err := sql.Open("postgres", connString)
	if err != nil {
		return err
	}
	defer conn.Close()
	err = conn.Ping()
	if err != nil {
		return err
	}

	var result bool
	if err := conn.QueryRow(pt.query).Scan(&result); err != nil {
		return err
	}
	if !result {
		return fmt.Errorf("the result of executing the query is false for pod %s", pt.targetHost)
	}
	return nil
}

func getNativeTLSMode(tlsMode string) string {
	switch tlsMode {
	case "SkipVerification":
		return "require"
	case "VerifyCA":
		return "verify-ca"
	case "VerifyAll":
		return "verify-full"
	case "Disabled":
		return "disable"
	default:
		return "require"
	}
}

type PostgreSQLCredentials struct {
	TlsMode    string
	User       string
	Password   string
	ClientCert string
	ClientKey  string
	CaCert     string
}
