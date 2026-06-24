package kube

import (
	"fmt"
	"reflect"

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

	crdVal := parser.CustomResourceDefinitions[groupKinds[0]]
	return &crdVal, nil
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
	flat := parser.FlattenedSchemata[ident]

	return &flat, nil
}
