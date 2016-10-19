package candyjs

import "fmt"

const (
	// ErrorCodeUndefinedProperty is returned when accessing an invalid property.
	ErrorCodeUndefinedProperty = "candyjs:undefinedproperty"
	// ErrorCodePackageNotFound is returned when a package cannot be found, usually this
	// happend when a PackagePusher function was not registered using
	// RegisterPackagePusher.
	ErrorCodePackageNotFound = "candyjs:packagenotfound"
)

// Error represents an error returned by candy JS
type Error interface {
	error
	Code() string
}

// ErrorCode returns the candy error code for this error.
// Returns "" if the error is not a candyjs error.
func ErrorCode(err error) string {
	cerr, _ := err.(Error)
	if cerr != nil {
		return cerr.Code()
	}
	return ""
}

type candyError struct {
	code string
	msg  string
}

func (e *candyError) Error() string {
	return e.msg
}

func (e *candyError) String() string {
	return e.code + ": " + e.msg
}

func (e *candyError) Code() string {
	return e.code
}

var _ Error = (*candyError)(nil)

func errorf(code string, msg string, args ...interface{}) error {
	return &candyError{code: code, msg: fmt.Sprintf(msg, args...)}
}
