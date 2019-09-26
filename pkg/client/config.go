package client

import (
	"context"
	"time"
)

type Config struct {
	Endpoint string

	Insecure bool

	// DialTimeout is the timeout for failing to establish a connection.
	DialTimeout time.Duration

	// Context is the default client context; it can be used to cancel grpc dial out and
	// other operations that do not have an explicit context.
	Context context.Context
}
