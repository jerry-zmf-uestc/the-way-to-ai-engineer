package runtime

import "context"

type Server interface {
	Addr() string
	Shutdown(context.Context) error
}
