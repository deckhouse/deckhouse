// Copyright 2021 Flant JSC
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

package d8crypto

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"fmt"
)

type Keypair struct {
	Pub  string `json:"pub"`
	Priv string `json:"priv"`
}

func GenKeypair() error {
	public, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return err
	}

	privateBytes, err := x509.MarshalPKCS8PrivateKey(private)
	if err != nil {
		return err
	}
	privateBlock := &pem.Block{
		Type:  "ED25519 PRIVATE KEY",
		Bytes: privateBytes,
	}
	privatePEM := pem.EncodeToMemory(privateBlock)

	publicBytes, err := x509.MarshalPKIXPublicKey(public)
	if err != nil {
		return err
	}
	publicBlock := &pem.Block{
		Type:  "ED25519 PUBLIC KEY",
		Bytes: publicBytes,
	}
	publicPEM := pem.EncodeToMemory(publicBlock)

	keypairJSON, err := json.Marshal(Keypair{
		Pub:  string(publicPEM),
		Priv: string(privatePEM),
	})
	if err != nil {
		return err
	}

	fmt.Println(string(keypairJSON))
	return nil
}
