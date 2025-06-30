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

package docbuilder

import (
	"archive/tar"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	coordv1 "k8s.io/api/coordination/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/retry"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha1"
	"github.com/deckhouse/deckhouse/go_lib/d8env"
	"github.com/deckhouse/deckhouse/go_lib/dependency"
	"github.com/deckhouse/deckhouse/go_lib/module"
	docsbuilder "github.com/deckhouse/deckhouse/go_lib/module/docs-builder"
	"github.com/deckhouse/deckhouse/pkg/log"
)

const defaultDocumentationCheckInterval = 10 * time.Second

type reconciler struct {
	client               client.Client
	downloadedModulesDir string

	dc          dependency.Container
	docsBuilder *docsbuilder.Client

	logger *log.Logger
}

func RegisterController(mgr manager.Manager, dc dependency.Container, logger *log.Logger) error {
	r := &reconciler{
		client:               mgr.GetClient(),
		downloadedModulesDir: d8env.GetDownloadedModulesDir(),
		dc:                   dependency.NewDependencyContainer(),
		docsBuilder:          docsbuilder.NewClient(dc.GetHTTPClient()),
		logger:               logger,
	}

	ctr, err := controller.New("module-documentation", mgr, controller.Options{
		MaxConcurrentReconciles: 1, // don't use concurrent reconciles here, because docs-builder doesn't support multiply requests at once
		CacheSyncTimeout:        15 * time.Minute,
		NeedLeaderElection:      ptr.To(false),
		Reconciler:              r,
	})
	if err != nil {
		return err
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.ModuleDocumentation{}).
		Watches(&coordv1.Lease{}, handler.EnqueueRequestsFromMapFunc(r.enqueueLeaseMapFunc), builder.WithPredicates(predicate.Funcs{
			CreateFunc: func(event event.CreateEvent) bool {
				ns := event.Object.GetNamespace()
				if ns != "d8-system" {
					return false
				}

				var hasLabel bool
				for label := range event.Object.GetLabels() {
					if label == "deckhouse.io/documentation-builder-sync" {
						hasLabel = true
						break
					}
				}

				return hasLabel
			},
		})).
		WithEventFilter(predicate.GenerationChangedPredicate{}).
		Complete(ctr)
}

func (r *reconciler) enqueueLeaseMapFunc(ctx context.Context, _ client.Object) []reconcile.Request {
	requests := make([]reconcile.Request, 0)

	err := retry.OnError(retry.DefaultRetry, apierrors.IsServiceUnavailable, func() error {
		mdl := new(v1alpha1.ModuleDocumentationList)

		err := r.client.List(ctx, mdl)
		if err != nil {
			return err
		}

		requests = make([]reconcile.Request, 0, len(mdl.Items))

		for _, md := range mdl.Items {
			requests = append(requests, reconcile.Request{NamespacedName: types.NamespacedName{Name: md.GetName()}})
		}

		return nil
	})
	if err != nil {
		log.Error("create mapping for lease failed", log.Err(err))
	}

	return requests
}

const documentationExistsFinalizer = "modules.deckhouse.io/documentation-exists"

func (r *reconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	var result ctrl.Result
	md := new(v1alpha1.ModuleDocumentation)

	if err := r.client.Get(ctx, req.NamespacedName, md); err != nil {
		return result, client.IgnoreNotFound(err)
	}

	if !md.DeletionTimestamp.IsZero() {
		if !controllerutil.ContainsFinalizer(md, documentationExistsFinalizer) {
			return result, nil
		}

		return r.deleteReconcile(ctx, md)
	}

	return r.createOrUpdateReconcile(ctx, md)
}

func (r *reconciler) deleteReconcile(ctx context.Context, md *v1alpha1.ModuleDocumentation) (ctrl.Result, error) {
	var result ctrl.Result

	// get addresses from cluster, not status, because them more actual
	addrs, err := r.getDocsBuilderAddresses(ctx)
	if err != nil {
		return result, fmt.Errorf("get docs builder addresses: %w", err)
	}

	if len(addrs) == 0 {
		// no endpoints for doc builder
		return result, nil
	}

	now := metav1.NewTime(r.dc.GetClock().Now().UTC())

	for _, addr := range addrs {
		if err = r.deleteDocumentation(ctx, addr, md.Name); err == nil {
			continue
		}

		delErr := fmt.Errorf("delete documentation: %w", err)

		_, idx := md.GetConditionByAddress(addr)
		if idx < 0 {
			continue
		}

		md.Status.Conditions[idx].Type = v1alpha1.TypeError
		md.Status.Conditions[idx].Message = delErr.Error()
		md.Status.Conditions[idx].LastTransitionTime = now

		if err = r.client.Status().Update(ctx, md); err != nil {
			r.logger.Error("update status when delete documentation", log.Err(err))

			return result, fmt.Errorf("update status when delete documentation: %w", errors.Join(delErr, err))
		}

		return result, delErr
	}

	controllerutil.RemoveFinalizer(md, documentationExistsFinalizer)
	if err = r.client.Update(ctx, md); err != nil {
		r.logger.Error("update finalizer", log.Err(err))

		return result, fmt.Errorf("update finalizer: %w", err)
	}

	return result, nil
}

func (r *reconciler) createOrUpdateReconcile(ctx context.Context, md *v1alpha1.ModuleDocumentation) (ctrl.Result, error) {
	var result ctrl.Result
	moduleName := md.Name

	r.logger.Info("Updating documentation for module", slog.String("name", moduleName))
	addrs, err := r.getDocsBuilderAddresses(ctx)
	if err != nil {
		return result, fmt.Errorf("get docs builder addresses: %w", err)
	}

	if len(addrs) == 0 {
		// no endpoints for doc builder
		return result, nil
	}

	b := new(bytes.Buffer)

	r.logger.Debug("Getting module's documentation locally", slog.String("moduleName", moduleName))
	fetchModuleErr := r.getDocumentationFromModuleDir(md.Spec.Path, b)

	var rendered int
	now := metav1.NewTime(r.dc.GetClock().Now().UTC())

	mdCopy := md.DeepCopy()
	mdCopy.Status.Conditions = make([]v1alpha1.ModuleDocumentationCondition, 0, len(addrs))

	for _, addr := range addrs {
		cond, condIdx := md.GetConditionByAddress(addr)
		// TODO: add function for compare
		if condIdx >= 0 &&
			cond.Version == md.Spec.Version &&
			cond.Checksum == md.Spec.Checksum &&
			cond.Type == v1alpha1.TypeRendered {
			// documentation is rendered for this builder
			mdCopy.Status.Conditions = append(mdCopy.Status.Conditions, cond)
			rendered++
			continue
		}

		cond = v1alpha1.ModuleDocumentationCondition{
			Address:            addr,
			Version:            md.Spec.Version,
			Checksum:           md.Spec.Checksum,
			LastTransitionTime: now,
		}

		if fetchModuleErr != nil {
			cond.Type = v1alpha1.TypeError
			cond.Message = fmt.Sprintf("Error occurred while fetching the documentation: %s. Please fix the module's docs or restart the Deckhouse to restore the module", fetchModuleErr)
			mdCopy.Status.Conditions = append(mdCopy.Status.Conditions, cond)
			continue
		}

		err = r.buildDocumentation(ctx, bytes.NewReader(b.Bytes()), addr, moduleName, md.Spec.Version)
		if err != nil {
			cond.Type = v1alpha1.TypeError
			cond.Message = err.Error()
		} else {
			rendered++
			cond.Type = v1alpha1.TypeRendered
			cond.Message = ""
		}

		mdCopy.Status.Conditions = append(mdCopy.Status.Conditions, cond)
	}

	switch {
	case rendered == 0:
		mdCopy.Status.RenderResult = v1alpha1.ResultError

	case rendered == len(addrs):
		mdCopy.Status.RenderResult = v1alpha1.ResultRendered

	default:
		mdCopy.Status.RenderResult = v1alpha1.ResultPartially
	}

	if err = r.client.Status().Patch(ctx, mdCopy, client.MergeFrom(md)); err != nil {
		return result, err
	}

	if mdCopy.Status.RenderResult != v1alpha1.ResultRendered {
		return ctrl.Result{RequeueAfter: defaultDocumentationCheckInterval}, nil
	}

	if !controllerutil.ContainsFinalizer(mdCopy, documentationExistsFinalizer) {
		controllerutil.AddFinalizer(mdCopy, documentationExistsFinalizer)
		if err = r.client.Update(ctx, mdCopy); err != nil {
			r.logger.Error("update finalizer", log.Err(err))

			return result, err
		}
	}

	return result, nil
}

func (r *reconciler) getDocsBuilderAddresses(ctx context.Context) ([]string, error) {
	var leasesList coordv1.LeaseList
	if err := r.client.List(ctx, &leasesList, client.InNamespace("d8-system"), client.HasLabels{"deckhouse.io/documentation-builder-sync"}); err != nil {
		return nil, fmt.Errorf("list leases: %w", err)
	}

	addresses := make([]string, 0, len(leasesList.Items))
	for _, lease := range leasesList.Items {
		if lease.Spec.HolderIdentity == nil {
			continue
		}

		// a stale lease found
		if lease.Spec.RenewTime.Add(time.Duration(*lease.Spec.LeaseDurationSeconds) * time.Second).Before(r.dc.GetClock().Now()) {
			continue
		}

		addresses = append(addresses, "http://"+*lease.Spec.HolderIdentity)
	}

	return addresses, nil
}

func (r *reconciler) getDocumentationFromModuleDir(modulePath string, buf *bytes.Buffer) error {
	moduleDir := path.Join(r.downloadedModulesDir, modulePath) + "/"

	dir, err := os.Stat(moduleDir)
	if err != nil {
		return err
	}

	if !dir.IsDir() {
		return fmt.Errorf("%s isn't a directory", moduleDir)
	}

	tw := tar.NewWriter(buf)
	defer tw.Close()

	err = filepath.Walk(moduleDir, func(file string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if filepath.Ext(file) == ".go" {
			return nil
		}

		if !module.IsDocsPath(strings.TrimPrefix(file, moduleDir)) {
			return nil
		}

		header, err := tar.FileInfoHeader(info, info.Name())
		if err != nil {
			return err
		}

		header.Name = strings.TrimPrefix(file, moduleDir)

		if err = tw.WriteHeader(header); err != nil {
			return err
		}

		if info.IsDir() {
			return nil
		}

		f, err := os.Open(file)
		if err != nil {
			return err
		}
		defer f.Close()

		r.logger.Debug("copy file", slog.String("path", file))

		if _, err = io.Copy(tw, f); err != nil {
			return err
		}

		return nil
	})
	if err != nil {
		return fmt.Errorf("read to buffer: %w", err)
	}

	return nil
}

func (r *reconciler) buildDocumentation(ctx context.Context, docsArchive io.Reader, baseAddr, moduleName, moduleVersion string) error {
	if err := r.docsBuilder.SendDocumentation(ctx, baseAddr, moduleName, moduleVersion, docsArchive); err != nil {
		return fmt.Errorf("send documentation: %w", err)
	}

	if err := r.docsBuilder.BuildDocumentation(ctx, baseAddr); err != nil {
		return fmt.Errorf("build documentation: %w", err)
	}

	return nil
}

func (r *reconciler) deleteDocumentation(ctx context.Context, baseAddr, moduleName string) error {
	if err := r.docsBuilder.DeleteDocumentation(ctx, baseAddr, moduleName); err != nil {
		return fmt.Errorf("delete documentation: %w", err)
	}

	if err := r.docsBuilder.BuildDocumentation(ctx, baseAddr); err != nil {
		return fmt.Errorf("build documentation: %w", err)
	}

	return nil
}
