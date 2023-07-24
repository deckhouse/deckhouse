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

package image

import (
	"io"

	"github.com/containers/image/v5/copy"
	"github.com/containers/image/v5/types"
)

type CopyOption func(*copyOptions)

type ListOption func(*listOptions)

type copyOptions struct {
	copyOptions *copy.Options
	dryRun      bool
}

func WithPreserveDigests() CopyOption {
	return func(o *copyOptions) {
		o.copyOptions.PreserveDigests = true
	}
}

func WithCopyAllImages() CopyOption {
	return func(o *copyOptions) {
		o.copyOptions.ImageListSelection = copy.CopyAllImages
	}
}

func WithOutput(w io.Writer) CopyOption {
	return func(o *copyOptions) {
		o.copyOptions.ReportWriter = w
	}
}

func WithDestInsecure() CopyOption {
	return func(o *copyOptions) {
		if o.copyOptions.DestinationCtx == nil {
			o.copyOptions.DestinationCtx = &types.SystemContext{}
		}
		o.copyOptions.DestinationCtx.DockerInsecureSkipTLSVerify = types.OptionalBoolTrue
	}
}

func withSourceAuth(cfg *types.DockerAuthConfig) CopyOption {
	return func(o *copyOptions) {
		if cfg == nil {
			return
		}
		if o.copyOptions.SourceCtx == nil {
			o.copyOptions.SourceCtx = &types.SystemContext{}
		}
		o.copyOptions.SourceCtx.DockerAuthConfig = cfg
	}
}

func withDestAuth(cfg *types.DockerAuthConfig) CopyOption {
	return func(o *copyOptions) {
		if cfg == nil {
			return
		}
		if o.copyOptions.DestinationCtx == nil {
			o.copyOptions.DestinationCtx = &types.SystemContext{}
		}
		o.copyOptions.DestinationCtx.DockerAuthConfig = cfg
	}
}

func WithDryRun() CopyOption {
	return func(o *copyOptions) {
		o.dryRun = true
	}
}

type listOptions struct {
	sysCtx *types.SystemContext
}

func withAuth(cfg *types.DockerAuthConfig) ListOption {
	return func(lo *listOptions) {
		if cfg == nil {
			return
		}
		if lo.sysCtx == nil {
			lo.sysCtx = &types.SystemContext{}
		}
		lo.sysCtx.DockerAuthConfig = cfg
	}
}

func WithInsecure() ListOption {
	return func(lo *listOptions) {
		if lo.sysCtx == nil {
			lo.sysCtx = &types.SystemContext{}
		}
		lo.sysCtx.DockerInsecureSkipTLSVerify = types.OptionalBoolTrue
	}
}
