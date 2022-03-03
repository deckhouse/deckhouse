/*
Copyright 2022 Flant JSC

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

package v1

type Provider struct {
	CloudID            string `json:"cloudID" yaml:"cloudID"`
	FolderID           string `json:"folderID" yaml:"folderID"`
	ServiceAccountJSON string `json:"serviceAccountJSON" yaml:"serviceAccountJSON"`
}

type ServiceAccount struct {
	ID               string `json:"id" yaml:"id"`
	ServiceAccountID string `json:"service_account_id" yaml:"service_account_id"`
	CreatedAt        string `json:"created_at" yaml:"created_at"`
	KeyAlgorithm     string `json:"key_algorithm" yaml:"key_algorithm"`
	PublicKey        string `json:"public_key" yaml:"public_key"`
	PrivateKey       string `json:"private_key" yaml:"private_key"`
}

type APIKeyResponse struct {
	ID               string `json:"id"`
	ServiceAccountID string `json:"serviceAccountId"`
	CreatedAt        string `json:"createdAt"`
	Description      string `json:"description"`
}

type APIKeyCreationResponse struct {
	APIKey APIKeyResponse `json:"apiKey"`
	Secret string         `json:"secret"`
}

type APIKeyCreationRequest struct {
	ServiceAccountID string `json:"serviceAccountId"`
	Description      string `json:"description"`
}

type IAMTokenCreationResponse struct {
	IAMToken string `json:"iamToken"`
}

type IAMTokenCreationRequest struct {
	JWT string `json:"jwt"`
}
