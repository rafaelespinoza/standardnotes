// package errs defines commonly occurring errors as interfaces as well as
// simple type checking functions. The goal is to loosely couple the caller from
// the specific kind of returned error while allowing some preservation of the
// error context.
package errs

type (
	Validation interface {
		Validation() bool
	}

	NotFound interface {
		NotFound() bool
	}
)

func ValidationError(e error) bool {
	err, ok := e.(Validation)
	return ok && err.Validation()
}

func NotFoundError(e error) bool {
	err, ok := e.(NotFound)
	return ok && err.NotFound()
}
