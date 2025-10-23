// Copyright 2024 Flant JSC
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

package context

import (
	"crypto/rand"
	"crypto/rsa"
	"encoding/hex"
	"encoding/pem"
	"fmt"

	"github.com/GehirnInc/crypt"
	_ "github.com/GehirnInc/crypt/sha512_crypt"
	gossh "github.com/deckhouse/lib-gossh"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	v1 "github.com/deckhouse/deckhouse/dhctl/pkg/apis/v1"
	"github.com/deckhouse/deckhouse/dhctl/pkg/global"
)

type NodeUserCredentials struct {
	Name string `json:"name"`
	// PEM encoded private key
	PrivateKey string `json:"privateKey"`
	// Password in plain text
	Password string `json:"password"`
}

func GenerateNodeUser() (*v1.NodeUser, *NodeUserCredentials, error) {
	passwordBytes := make([]byte, 16)
	_, err := rand.Read(passwordBytes)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to generate random password: %w", err)
	}

	password := []byte(hex.EncodeToString(passwordBytes))

	passwordHash, err := generatePasswordHash(password)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to generate password hash: %w", err)
	}

	privateKey, err := generatePrivateKey(2048)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to generate private key: %w", err)
	}

	privateKeyPem, err := gossh.MarshalPrivateKeyWithPassphrase(privateKey, "", password)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to marshal private key to PEM: %w", err)
	}

	publicKey, err := gossh.NewPublicKey(privateKey.Public())
	if err != nil {
		return nil, nil, fmt.Errorf("failed to generate public key: %w", err)
	}

	return &v1.NodeUser{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "deckhouse.io/v1",
				Kind:       "NodeUser",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name: global.ConvergeNodeUserName,
			},
			Spec: v1.NodeUserSpec{
				UID: global.ConvergeNodeUserUID,
				SSHPublicKeys: []string{
					string(gossh.MarshalAuthorizedKey(publicKey)),
				},
				PasswordHash: passwordHash,
				IsSudoer:     true,
				NodeGroups:   []string{global.MasterNodeGroupName},
			},
		}, &NodeUserCredentials{
			Name:       global.ConvergeNodeUserName,
			PrivateKey: string(pem.EncodeToMemory(privateKeyPem)),
			Password:   string(password),
		}, nil
}

func generatePasswordHash(password []byte) (string, error) {
	passwordHash, err := crypt.SHA512.New().Generate(password, nil)
	if err != nil {
		return "", fmt.Errorf("failed to generate password hash: %w", err)
	}

	return passwordHash, nil
}

func generatePrivateKey(bitSize int) (*rsa.PrivateKey, error) {
	privateKey, err := rsa.GenerateKey(rand.Reader, bitSize)
	if err != nil {
		return nil, fmt.Errorf("failed to generate private key: %w", err)
	}

	return privateKey, nil
}
