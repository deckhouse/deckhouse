// Copyright 2023 Flant JSC
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

package mirror

import (
	"context"
	"fmt"
	"time"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/empty"
	"github.com/google/go-containerregistry/pkg/v1/remote"
)

func ValidateReadAccessForImage(imageTag string, authProvider authn.Authenticator, insecure bool) error {
	nameOpts, remoteOpts := MakeRemoteRegistryRequestOptions(authProvider, insecure)
	ref, err := name.ParseReference(imageTag, nameOpts...)
	if err != nil {
		return fmt.Errorf("Parse registry address: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	remoteOpts = append(remoteOpts, remote.WithContext(ctx))
	_, err = remote.Head(ref, remoteOpts...)
	if err != nil {
		return err
	}

	return nil
}

func ValidateWriteAccessForRepo(repo string, authProvider authn.Authenticator, insecure bool) error {
	nameOpts, remoteOpts := MakeRemoteRegistryRequestOptions(authProvider, insecure)
	ref, err := name.NewTag(repo+":dhctlWriteCheck", nameOpts...)
	if err != nil {
		return err
	}

	if err = remote.Write(ref, empty.Image, remoteOpts...); err != nil {
		return err
	}

	if err = remote.Delete(ref, remoteOpts...); err != nil {
		return fmt.Errorf("Could not clean up image %q after write-testing registry permissions: %w", ref.String(), err)
	}

	return nil
}

func MakeRemoteRegistryRequestOptions(authProvider authn.Authenticator, insecure bool) ([]name.Option, []remote.Option) {
	n, r := make([]name.Option, 0), make([]remote.Option, 0)
	if insecure {
		n = append(n, name.Insecure)
	}
	if authProvider != nil && authProvider != authn.Anonymous {
		r = append(r, remote.WithAuth(authProvider))
	}

	return n, r
}

func MakeRemoteRegistryRequestOptionsFromMirrorContext(mirrorCtx *Context) ([]name.Option, []remote.Option) {
	return MakeRemoteRegistryRequestOptions(mirrorCtx.RegistryAuth, mirrorCtx.Insecure)
}
