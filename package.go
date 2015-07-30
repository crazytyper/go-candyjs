package candyjs

import "errors"

// PackagePusher should be a function capable of register all functions and
// types contained on a golang packages. This functions are generated by the
// go generate tool `candyjs` a example of this header is:
//   //go:generate candyjs import time
type PackagePusher func(ctx *Context)

// PackageNotFound error is throw when a package cannot be found, usually this
// happend when a PackagePusher function was not registered using
// RegisterPackagePusher.
var ErrPackageNotFound = errors.New("Unable to find the requested package")
var pushers = make(map[string]PackagePusher, 0)

// RegisterPackagePusher registers a PackagePusher into the global storage, this
// storage is a private map defined on the candyjs package. The pushers are
// launch by the function PushGlobalPackage.
func RegisterPackagePusher(pckgName string, f PackagePusher) {
	pushers[pckgName] = f
}

// PushGlobalPackage all the functions and types from the given package using
// the pre-registered PackagePusher function.
func (ctx *Context) PushGlobalPackage(pckgName, alias string) error {
	ctx.Duktape.PushGlobalObject()

	err := ctx.pushPackage(pckgName)
	if err != nil {
		return err
	}

	ctx.Duktape.PutPropString(-2, alias)
	ctx.Duktape.Pop()

	return nil
}

func (ctx *Context) pushPackage(pckgName string) error {
	f, ok := pushers[pckgName]
	if !ok {
		return ErrPackageNotFound
	}

	f(ctx)

	return nil
}
