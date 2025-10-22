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

package app

import (
	"strconv"
	"time"

	"github.com/sirupsen/logrus"
	"gopkg.in/alecthomas/kingpin.v2"
)

var (
	ApiKey            = ""
	ApiKeyFilePath    = ""
	FolderID          = ""
	ListenAddress     = "127.0.0.1:9000"
	Services          = make([]string, 0)
	LoggerType        = loggerJSON
	LoggerLevel       = int(logrus.InfoLevel)
	AutoRenewIAMToken = 1 * time.Hour
)

func InitFlags(cmd *kingpin.Application) {
	cmd.Flag("api-key", "API key for service account").
		Envar("API_KEY").
		StringVar(&ApiKey)

	cmd.Flag("api-key-file", "API key file path for service account").
		Envar("API_KEY_PATH").
		StringVar(&ApiKeyFilePath)

	cmd.Flag("folder-id", "Yandex folder id").
		Envar("FOLDER_ID").
		Required().
		StringVar(&FolderID)

	cmd.Flag("listen-address", "Listen address for HTTP").
		Envar("LISTEN_ADDRESS").
		Default(ListenAddress).
		StringVar(&ListenAddress)

	cmd.Flag("logger-type", "Format logs output of a dhctl in different ways.").
		Envar("LOGGER_TYPE").
		Default(LoggerType).
		EnumVar(&LoggerType, loggerJSON, loggerSimple)

	cmd.Flag("v", "Logger verbosity").
		Envar("LOGGER_LEVEL").
		Default(strconv.Itoa(int(LoggerLevel))).
		IntVar(&LoggerLevel)

	cmd.Flag("services", "List services for '/metrics' path").
		Envar("HTTP_PORT").
		StringsVar(&Services)

	cmd.Flag("auto-renew-iam-token-period", "Period for renew yandex IAM-token for service account").
		Envar("HTTP_PORT").
		Default(AutoRenewIAMToken.String()).
		DurationVar(&AutoRenewIAMToken)

}
