//go:generate go tool mockgen -destination api_mocks_test.go -package http_test github.com/SPROG666/go-autumn-2026/api ServerInterface
//go:generate go tool mockgen -source contract.go -destination mocks_test.go -package http_test
package http

import "context"

type databasePinger interface {
	Ping(ctx context.Context) error
}
