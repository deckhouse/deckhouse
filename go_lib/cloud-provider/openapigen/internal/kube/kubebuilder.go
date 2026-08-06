/*
Copyright 2026 Flant JSC

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

package kube

import (
	"errors"
	"fmt"
	"reflect"
	"strings"

	"golang.org/x/tools/go/packages"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"sigs.k8s.io/controller-tools/pkg/crd"
	crdmarkers "sigs.k8s.io/controller-tools/pkg/crd/markers"
	"sigs.k8s.io/controller-tools/pkg/loader"
	"sigs.k8s.io/controller-tools/pkg/markers"
)

// GetCRDFromRoots builds a CustomResourceDefinition from one or more root types
// using the same pipeline as controller-gen. Each root must embed metav1.TypeMeta
// and metav1.ObjectMeta, and its package must carry +groupName and the type must
// carry +kubebuilder:object:root=true.
//
// If maxDescLen is non-nil, all description fields are trimmed to that length.
func GetCRDFromRoots(roots []any, maxDescLen *int) (*apiextensionsv1.CustomResourceDefinition, error) {
	reg := &markers.Registry{}
	if err := crdmarkers.Register(reg); err != nil {
		return nil, err
	}

	parser := &crd.Parser{
		Collector: &markers.Collector{
			Registry: reg,
		},
		Checker:                &loader.TypeChecker{},
		IgnoreUnexportedFields: true,
	}

	crd.AddKnownTypes(parser)

	// collect all loaded packages across roots (dedup by path)
	seen := make(map[string]struct{})
	var allPkgs []*loader.Package

	for _, root := range roots {
		rt := reflect.TypeOf(root)
		for rt.Kind() == reflect.Pointer {
			rt = rt.Elem()
		}
		pkgPath := rt.PkgPath()

		pkgs, err := loader.LoadRoots(pkgPath)
		if err != nil {
			return nil, fmt.Errorf("load package %s: %w", pkgPath, err)
		}
		for _, pkg := range pkgs {
			if _, ok := seen[pkg.PkgPath]; ok {
				continue
			}
			seen[pkg.PkgPath] = struct{}{}
			allPkgs = append(allPkgs, pkg)
			parser.NeedPackage(pkg)
		}
	}

	metav1Pkg := crd.FindMetav1(allPkgs)
	if metav1Pkg == nil {
		return nil, fmt.Errorf("metav1 package not found; ensure root types embed metav1.TypeMeta and metav1.ObjectMeta")
	}

	groupKinds := crd.FindKubeKinds(parser, metav1Pkg)
	if len(groupKinds) == 0 {
		return nil, fmt.Errorf("no CRD root types found; ensure types embed metav1.TypeMeta and metav1.ObjectMeta and carry +kubebuilder:object:root=true")
	}

	for _, gk := range groupKinds {
		parser.NeedCRDFor(gk, maxDescLen)
	}

	if err := checkPackageErrors(parsedPackages(parser, allPkgs)); err != nil {
		return nil, err
	}

	crdVal := parser.CustomResourceDefinitions[groupKinds[0]]
	return &crdVal, nil
}

const (
	// namedTypeMarkerError is the one class of controller-tools failure this generator
	// tolerates: a validation marker on a field whose type is a named type. At that point
	// the field schema is a bare $ref with an empty type, so controller-tools refuses the
	// marker and reports `found type ""`.
	//
	// The constraint IS LOST — nothing re-applies it. Compensating deckhouse markers were
	// considered and deliberately rejected: the supported alternatives are to give the field
	// a builtin type, to move the marker onto the named type itself (it then applies to every
	// field of that type), or to express the constraint as CEL via
	// +kubebuilder:validation:XValidation, which controller-tools does apply through a $ref.
	//
	// Upgrading controller-tools to v0.20.1+ removes the limitation, but the version here is
	// pinned to the Kubernetes 1.34 line on purpose. Revisit when the default moves to 1.35.
	namedTypeMarkerError = `found type ""`
)

// checkPackageErrors turns controller-tools marker failures into generation failures.
//
// Without this, a marker controller-tools refuses is dropped silently and the constraint
// simply disappears from the generated CRD or config-values schema: a real loss of
// in-cluster validation that looks like a successful generation.
func checkPackageErrors(pkgs []*loader.Package) error {
	var reported []error
	for _, pkg := range pkgs {
		for _, pkgErr := range pkg.Errors {
			// Marker and schema failures are recorded as UnknownError (loader.PositionedError).
			// Parse and type-check errors are a different matter: controller-tools type-checks
			// the target package in isolation, so imports from other Go modules routinely come
			// out as "undefined: <pkg>" without affecting the generated schema.
			if pkgErr.Kind != packages.UnknownError {
				continue
			}

			if strings.Contains(pkgErr.Error(), namedTypeMarkerError) {
				continue
			}

			reported = append(reported, pkgErr)
		}
	}

	if len(reported) == 0 {
		return nil
	}

	return fmt.Errorf("controller-tools rejected markers, constraints would be lost silently: %w", errors.Join(reported...))
}

// parsedPackages returns the root packages plus every package the parser pulled in
// while resolving types. Marker errors are recorded on the package that declares the
// offending field, which is routinely a package the roots merely import: checking the
// roots alone lets constraints on shared types disappear without a word.
func parsedPackages(parser *crd.Parser, roots []*loader.Package) []*loader.Package {
	seen := make(map[*loader.Package]struct{}, len(roots)+len(parser.Types))
	all := make([]*loader.Package, 0, len(roots)+len(parser.Types))

	add := func(pkg *loader.Package) {
		if pkg == nil {
			return
		}
		if _, ok := seen[pkg]; ok {
			return
		}
		seen[pkg] = struct{}{}
		all = append(all, pkg)
	}

	for _, pkg := range roots {
		add(pkg)
	}
	for ident := range parser.Types {
		add(ident.Package)
	}

	return all
}

func GetJSONSchemaPropsFromDefaultMarkers(root any) (*apiextensionsv1.JSONSchemaProps, error) {
	reg := &markers.Registry{}

	err := crdmarkers.Register(reg)
	if err != nil {
		return nil, err
	}

	return getJSONSchemaProps(root, reg)
}

func getJSONSchemaProps(root any, reg *markers.Registry) (*apiextensionsv1.JSONSchemaProps, error) {
	rt := reflect.TypeOf(root)
	if rt.Kind() == reflect.Pointer {
		rt = rt.Elem()
	}
	if rt.Kind() != reflect.Struct {
		return nil, fmt.Errorf("expected struct or pointer to struct, got %s", rt.Kind())
	}

	parser := &crd.Parser{
		Collector: &markers.Collector{
			Registry: reg,
		},
		Checker:                &loader.TypeChecker{},
		IgnoreUnexportedFields: true,
	}

	rtPkg, err := loader.LoadRoots(rt.PkgPath())
	if err != nil {
		return nil, err
	}

	if len(rtPkg) < 1 {
		return nil, fmt.Errorf("could not find package %s", rt.PkgPath())
	}

	ident := crd.TypeIdent{Package: rtPkg[0], Name: rt.Name()}

	parser.NeedFlattenedSchemaFor(ident)

	if err := checkPackageErrors(parsedPackages(parser, rtPkg)); err != nil {
		return nil, err
	}

	flat := parser.FlattenedSchemata[ident]

	return &flat, nil
}
