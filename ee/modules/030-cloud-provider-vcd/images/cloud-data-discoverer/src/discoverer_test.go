/*
Copyright 2023 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package main

import (
	"reflect"
	"testing"

	"k8s.io/apimachinery/pkg/api/resource"

	"github.com/deckhouse/deckhouse/go_lib/cloud-data/apis/v1alpha1"
)

func TestRemoveDuplicatesStrings(t *testing.T) {
	tests := []struct {
		args []string
		want []string
	}{
		{
			args: []string{"1", ""},
			want: []string{"1"},
		},
		{
			args: []string{"1", "1", "", ""},
			want: []string{"1"},
		},
		{
			args: []string{"1", "2", "3", "1", "", ""},
			want: []string{"1", "2", "3"},
		},
		{
			args: []string{"one", "two", "three", "four", "two", "four", "five"},
			want: []string{"five", "four", "one", "three", "two"},
		},
	}

	for _, test := range tests {
		t.Run("", func(t *testing.T) {
			if got := removeDuplicatesStrings(test.args); !reflect.DeepEqual(got, test.want) {
				t.Errorf("removeDuplicatesStrings() = %v, want %v", got, test.want)
			}
		})
	}
}

func TestRemoveDuplicatesStorageProfiles(t *testing.T) {
	tests := []struct {
		args []v1alpha1.VCDStorageProfile
		want []v1alpha1.VCDStorageProfile
	}{
		{
			args: []v1alpha1.VCDStorageProfile{
				{
					Name:                    "1",
					IsEnabled:               true,
					IsDefaultStorageProfile: true,
				},
				{
					Name:                    "3",
					IsEnabled:               true,
					IsDefaultStorageProfile: false,
				},
				{
					Name:                    "2",
					IsEnabled:               true,
					IsDefaultStorageProfile: false,
				},
				{
					Name:                    "",
					IsEnabled:               true,
					IsDefaultStorageProfile: false,
				},
			},
			want: []v1alpha1.VCDStorageProfile{
				{
					Name:                    "1",
					IsEnabled:               true,
					IsDefaultStorageProfile: true,
				},
				{
					Name:                    "2",
					IsEnabled:               true,
					IsDefaultStorageProfile: false,
				},
				{
					Name:                    "3",
					IsEnabled:               true,
					IsDefaultStorageProfile: false,
				},
			},
		},
	}

	for _, test := range tests {
		t.Run("", func(t *testing.T) {
			if got := removeDuplicatesStorageProfiles(test.args); !reflect.DeepEqual(got, test.want) {
				t.Errorf("removeDuplicates() = %v, want %v", got, test.want)
			}
		})
	}
}

func TestRemoveDuplicatesInstanceTypes(t *testing.T) {
	tests := []struct {
		args []v1alpha1.InstanceType
		want []v1alpha1.InstanceType
	}{
		{
			args: []v1alpha1.InstanceType{
				{
					Name:   "1",
					CPU:    resource.MustParse("4"),
					Memory: resource.MustParse("4Gi"),
				},
				{
					Name:   "3",
					CPU:    resource.MustParse("2"),
					Memory: resource.MustParse("4Gi"),
				},
				{
					Name:   "2",
					CPU:    resource.MustParse("4"),
					Memory: resource.MustParse("8Gi"),
				},
				{
					Name:   "",
					CPU:    resource.MustParse("4"),
					Memory: resource.MustParse("4Gi"),
				},
			},
			want: []v1alpha1.InstanceType{
				{
					Name:   "1",
					CPU:    resource.MustParse("4"),
					Memory: resource.MustParse("4Gi"),
				},
				{
					Name:   "2",
					CPU:    resource.MustParse("4"),
					Memory: resource.MustParse("8Gi"),
				},
				{
					Name:   "3",
					CPU:    resource.MustParse("2"),
					Memory: resource.MustParse("4Gi"),
				},
			},
		},
	}

	for _, test := range tests {
		t.Run("", func(t *testing.T) {
			if got := removeDuplicatesInstanceTypes(test.args); !reflect.DeepEqual(got, test.want) {
				t.Errorf("removeDuplicates() = %v, want %v", got, test.want)
			}
		})
	}
}
