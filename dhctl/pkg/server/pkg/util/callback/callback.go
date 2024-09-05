package callback

import "errors"

func Call(fncs ...func() error) error {
	return NewCallback(fncs...).Call()
}

type Callback struct {
	Functions []func() error
}

func NewCallback(fncs ...func() error) *Callback {
	cb := &Callback{}
	for _, f := range fncs {
		cb.Add(f)
	}
	return cb
}

func (cb *Callback) Add(f func() error) {
	if f == nil {
		return
	}
	cb.Functions = append(cb.Functions, f)
}

func (cb *Callback) Call() (err error) {
	for _, f := range cb.Functions {
		err = errors.Join(err, f())
	}
	return
}

func (cb *Callback) AsFunc() func() error {
	return cb.Call
}
