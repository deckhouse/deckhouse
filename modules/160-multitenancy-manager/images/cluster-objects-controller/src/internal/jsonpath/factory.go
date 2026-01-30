package jsonpath

import (
	"github.com/theory/jsonpath"
)

type Factory interface {
	Path(expr string) (*jsonpath.Path, error)
}
