/*
Copyright 2024 Flant JSC

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

import (
	"crypto/rand"
	"crypto/rsa"
	"encoding/hex"
	"encoding/pem"
	"fmt"

	"github.com/GehirnInc/crypt"
	_ "github.com/GehirnInc/crypt/sha512_crypt"
	"github.com/deckhouse/lib-gossh"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/deckhouse/deckhouse/dhctl/pkg/global"
)

var NodeUserGVK = schema.GroupVersionResource{
	Group:    "deckhouse.io",
	Version:  "v1",
	Resource: "nodeusers",
}

// NodeUser is an system user on nodes.
type NodeUser struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              NodeUserSpec `json:"spec"`
}

type NodeUserSpec struct {
	UID           int      `json:"uid"`
	SSHPublicKeys []string `json:"sshPublicKeys,omitempty"`
	PasswordHash  string   `json:"passwordHash"`
	IsSudoer      bool     `json:"isSudoer"`
	NodeGroups    []string `json:"nodeGroups"`
}

type NodeUserCredentials struct {
	Name string `json:"name"`
	// PEM encoded private key
	PrivateKey string `json:"privateKey"`
	// Password in plain text
	Password string `json:"password"`

	PublicKey string `json:"publicKey"`

	NodeGroups []string `json:"nodeGroups"`
}

type NodeUserParams struct {
	Name       string
	UUID       int
	NodeGroups []string
}

func ConvergerNodeUser() NodeUserParams {
	return NodeUserParams{
		Name: global.ConvergeNodeUserName,
		UUID: global.ConvergeNodeUserUID,
		NodeGroups: []string{
			global.MasterNodeGroupName,
		},
	}
}

func ConvergerNodeUserExistsChecker(node v1.Node) bool {
	annotations := node.GetAnnotations()
	if len(annotations) == 0 {
		return false
	}

	val, ok := annotations[global.ConvergerNodeUserAnnotation]
	return ok && val == "true"
}

func GenerateNodeUser(params NodeUserParams) (*NodeUser, *NodeUserCredentials, error) {
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

	privateKeyPem, err := ssh.MarshalPrivateKeyWithPassphrase(privateKey, "", password)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to marshal private key to PEM: %w", err)
	}

	publicKey, err := ssh.NewPublicKey(privateKey.Public())
	if err != nil {
		return nil, nil, fmt.Errorf("failed to generate public key: %w", err)
	}

	nodeGroups := params.NodeGroups
	if len(nodeGroups) == 0 {
		nodeGroups = []string{"*"}
	}

	return &NodeUser{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "deckhouse.io/v1",
				Kind:       "NodeUser",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name: params.Name,
			},
			Spec: NodeUserSpec{
				UID: params.UUID,
				SSHPublicKeys: []string{
					string(ssh.MarshalAuthorizedKey(publicKey)),
				},
				PasswordHash: passwordHash,
				IsSudoer:     true,
				NodeGroups:   nodeGroups,
			},
		}, &NodeUserCredentials{
			Name:       params.Name,
			PrivateKey: string(pem.EncodeToMemory(privateKeyPem)),
			Password:   string(password),
			PublicKey:  string(ssh.MarshalAuthorizedKey(publicKey)),
			NodeGroups: params.NodeGroups,
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
