package pkga

import "something/pkgb"

type A struct {
	Dep *pkgb.B
}
