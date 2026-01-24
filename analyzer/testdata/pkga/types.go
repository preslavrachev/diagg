package pkga

import "testdata/pkgb"

type A struct {
	Dep *pkgb.B
}
