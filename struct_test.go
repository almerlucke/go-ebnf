package ebnf

import (
	"testing"
)

type TestTransformer interface {
	Transform(v int) int
}

type TestTransformFunction func(int) int

type Base struct {
	T TestTransformFunction
}

func (b *Base) Transform(v int) int {
	if b.T != nil {
		return b.T(v)
	}

	return v
}

type TestStruct struct {
	Base
	Other int
}

func TestBase(t *testing.T) {
	ts := &TestStruct{
		Base: Base{
			T: func(v int) int {
				return v + 1
			},
		},
		Other: 3,
	}

	var tss TestTransformer

	tss = ts
	t.Logf("trans %v\n", tss.Transform(4))
}
