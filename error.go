package fastcgi

import (
	"errors"
)

var (
	ErrorGetRequestId       = errors.New("unable to get request id")
	ErrorInvalidFcgiVertion = errors.New("invalid fcgi version")
)
