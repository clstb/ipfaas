package openfaas

import "fmt"

var (
	ErrInternalServerError = fmt.Errorf("internal server error")
	ErrNotFound            = fmt.Errorf("not found")
	ErrBadRequest          = fmt.Errorf("bad request")
	ErrUnknownResponse     = fmt.Errorf("unknown response")
)
