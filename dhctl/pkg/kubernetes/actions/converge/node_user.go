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

package converge

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/hex"
	"encoding/pem"
	"fmt"
	"time"

	"github.com/GehirnInc/crypt"
	_ "github.com/GehirnInc/crypt/sha512_crypt"
	sdk "github.com/deckhouse/module-sdk/pkg/utils"
	"golang.org/x/crypto/ssh"
	k8errors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/client"
	"github.com/deckhouse/deckhouse/dhctl/pkg/util/retry"
)

const (
	nodeUserName = "d8-dhctl-converger"
	nodeUserUID  = 64536
)

var errNodeUserNotFound = fmt.Errorf("NodeUser not found")

var nodeUserGVK = schema.GroupVersionResource{
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
}

func generateNodeUser() (*NodeUser, *NodeUserCredentials, error) {
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

	return &NodeUser{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "deckhouse.io/v1",
				Kind:       "NodeUser",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name: nodeUserName,
			},
			Spec: NodeUserSpec{
				UID: nodeUserUID,
				SSHPublicKeys: []string{
					string(ssh.MarshalAuthorizedKey(publicKey)),
				},
				PasswordHash: passwordHash,
				IsSudoer:     true,
				NodeGroups:   []string{MasterNodeGroupName},
			},
		}, &NodeUserCredentials{
			Name:       nodeUserName,
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

func createNodeUser(ctx context.Context, kubeClient client.KubeClient, nodeUser *NodeUser) error {
	nodeUserResource, err := sdk.ToUnstructured(nodeUser)
	if err != nil {
		return fmt.Errorf("failed to convert NodeUser to unstructured: %w", err)
	}

	return retry.NewLoop("Save dhctl converge NodeUser", 45, 10*time.Second).Run(func() error {
		_, err = kubeClient.Dynamic().Resource(nodeUserGVK).Create(ctx, nodeUserResource, metav1.CreateOptions{})
		if err != nil && !k8errors.IsAlreadyExists(err) {
			return fmt.Errorf("failed to create NodeUser: %w", err)
		}

		return nil
	})
}

func getNodeUser(ctx context.Context, kubeClient client.KubeClient) (*NodeUser, error) {
	var nodeUserUnstructured *unstructured.Unstructured

	err := retry.NewLoop("Get dhctl converge NodeUser", 45, 10*time.Second).Run(func() (err error) {
		nodeUserUnstructured, err = kubeClient.Dynamic().Resource(nodeUserGVK).Get(ctx, nodeUserName, metav1.GetOptions{})
		if err != nil {
			if k8errors.IsNotFound(err) {
				return errNodeUserNotFound
			}

			return fmt.Errorf("failed to get NodeUser: %w", err)
		}

		return nil
	})

	var nodeUser NodeUser

	err = sdk.FromUnstructured(nodeUserUnstructured, &nodeUser)
	if err != nil {
		return nil, fmt.Errorf("failed to convert unstructured to NodeUser: %w", err)
	}

	return &nodeUser, nil
}
