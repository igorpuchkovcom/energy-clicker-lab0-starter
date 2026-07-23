package mirrorscout

import (
	"context"
	"net"
	"net/http"
)

type Resolver interface {
	LookupHost(ctx context.Context, host string) ([]string, error)
}

type Dialer interface {
	DialContext(ctx context.Context, network, address string) (net.Conn, error)
}

type HTTPDoer interface {
	Do(request *http.Request) (*http.Response, error)
}
