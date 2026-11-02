package chain

import "context"

// Server abstracts one started Hertz or Kitex server.
type Server interface {
	Addr() string
	Shutdown(context.Context) error
}
