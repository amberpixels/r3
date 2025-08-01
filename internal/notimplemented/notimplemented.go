package notimplemented

import "errors"

var Err = errors.New("not implemented")

// Panic panics with a not implemented error and optional details.
func Panic(details ...string) {
	if len(details) > 0 {
		panic(Err.Error() + ": " + details[0])
	}
	panic(Err.Error())
}
