package experimental

import (
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"
	"k8s.io/utils/ptr"
)

func TestFilter(t *testing.T) {
	truePtr, falsePtr := ptr.To(true), ptr.To(false)

	cases := []struct {
		name   string
		flag   bool
		labels map[string]string
		want   *bool
		err    bool
	}{
		{"skip-non-experimental", false, map[string]string{"stage": "Stable"}, nil, false},
		{"deny-when-flag-false", false, map[string]string{"stage": "Experimental"}, falsePtr, true},
		{"allow-when-flag-true", true, map[string]string{"stage": "Experimental"}, truePtr, false},
		{"no-labels", false, nil, nil, false},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			e := Instance()
			_ = e.AddConstraint("allowExperimentalModules", strconv.FormatBool(tt.flag))

			got, err := e.Filter("foo", tt.labels)

			if tt.err {
				require.Error(t, err)
				require.Equal(t, *tt.want, *got)
			} else {
				require.NoError(t, err)
				require.Equal(t, tt.want, got)
			}
		})
	}
}
