package taints

import (
	"sort"

	v1 "k8s.io/api/core/v1"
)

type Map map[string]v1.Taint

func (m Map) Slice() Slice {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}

	sort.Strings(keys)

	res := make([]v1.Taint, 0, len(m))
	for _, k := range keys {
		res = append(res, m[k])
	}
	return res
}
