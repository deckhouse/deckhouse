package hooks

import (
	"encoding/json"
	"fmt"

	"github.com/Masterminds/semver/v3"
	"github.com/flant/addon-operator/sdk"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// DecodeDataFromSecret returns data section from Secret. If possible, top level keys are converted from JSON.
func DecodeDataFromSecret(obj *unstructured.Unstructured) (map[string]interface{}, error) {
	secret := new(v1.Secret)
	err := sdk.FromUnstructured(obj, secret)
	if err != nil {
		return nil, err
	}

	res := map[string]interface{}{}
	for k, v := range secret.Data {
		res[k] = string(v)
		// Try to load JSON from value.
		var jsonValue interface{}
		err := json.Unmarshal(v, &jsonValue)
		if err == nil {
			switch v := jsonValue.(type) {
			case map[string]interface{}:
				res[k] = v
			case []interface{}:
				res[k] = v
			case string:
				res[k] = v
				// This default will convert numbers into float64. It seems not ok for secret data.
				//default:
				//	res[k] = jsonValue
			}
		}
	}

	return res, nil
}

// SemverMajMin is a Go implementation of this bash snippet:
//   function semver::majmin() {
//     echo "$(echo $1 | cut -d. -f1,2)"
//   }
func SemverMajMin(ver *semver.Version) string {
	if ver == nil {
		return ""
	}
	return fmt.Sprintf("%d.%d", ver.Major(), ver.Minor())
}

func SemverMin(versions []*semver.Version) *semver.Version {
	if len(versions) == 0 {
		return nil
	}
	var res *semver.Version
	for i, ver := range versions {
		if res == nil || res.GreaterThan(ver) {
			res = versions[i]
		}
	}
	return res
}
