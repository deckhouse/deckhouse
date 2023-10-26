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

package main

import (
	"fmt"
	"github.com/flant/doc_builder/pkg/hugo"
	"github.com/flant/doc_builder/pkg/k8s"
	"k8s.io/klog/v2"
	"net/http"
)

func newBuildHandler(src string, dst string, cmManager *k8s.ConfigmapManager) *buildHandler {
	return &buildHandler{
		src:       src,
		dst:       dst,
		cmManager: cmManager,
	}
}

type buildHandler struct {
	src       string
	dst       string
	cmManager *k8s.ConfigmapManager
}

func (b *buildHandler) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	err := b.build()
	if err != nil {
		klog.Error(err)
		http.Error(writer, "Internal server error", http.StatusInternalServerError)
		return
	}

	writer.WriteHeader(http.StatusOK)
}

func (b *buildHandler) build() error {
	flags := hugo.Flags{
		//TODO: Quiet:  true,
		Source: b.src,
	}

	err := hugo.Build(flags)
	if err != nil {
		return fmt.Errorf("hugo build: %w", err)
	}

	err = b.cmManager.Remove()
	if err != nil {
		return fmt.Errorf("remove cm: %w", err)
	}
	return nil
}
