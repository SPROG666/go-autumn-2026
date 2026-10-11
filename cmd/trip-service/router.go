package main

import (
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"

	"github.com/getkin/kin-openapi/openapi3filter"
	"github.com/getkin/kin-openapi/routers"
	"github.com/go-chi/chi/v5"

	"github.com/SPROG666/go-autumn-2026/api"
	"github.com/SPROG666/go-autumn-2026/internal/httpapi"
)

func newRouter(logger *slog.Logger, handler *httpapi.Handler) (http.Handler, error) {
	spec, err := api.GetSpec()
	if err != nil {
		return nil, fmt.Errorf("load embedded OpenAPI: %w", err)
	}
	router := chi.NewRouter()
	router.NotFound(func(w http.ResponseWriter, r *http.Request) {
		handler.Problem(w, r, http.StatusNotFound, "route_not_found", "route-not-found", "Route not found", "Requested route was not found")
	})
	router.MethodNotAllowed(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.RawPath
		if path == "" {
			path = r.URL.Path
		}
		var allowed []string
		for _, method := range []string{
			http.MethodGet, http.MethodHead, http.MethodPost, http.MethodPut,
			http.MethodPatch, http.MethodDelete, http.MethodConnect,
			http.MethodOptions, http.MethodTrace,
		} {
			if router.Match(chi.NewRouteContext(), method, path) {
				allowed = append(allowed, method)
			}
		}
		w.Header().Set("Allow", strings.Join(allowed, ", "))
		handler.Problem(w, r, http.StatusMethodNotAllowed, "method_not_allowed", "method-not-allowed", "Method not allowed", "HTTP method is not allowed for this route")
	})
	validate := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			pattern := chi.RouteContext(r.Context()).RoutePattern()
			if pattern == "/health" || pattern == "/ready" {
				next.ServeHTTP(w, r)
				return
			}
			path := spec.Paths.Value(pattern)
			if path == nil || path.Operations()[r.Method] == nil {
				logger.ErrorContext(r.Context(), "route missing from embedded OpenAPI", "path", pattern)
				handler.Problem(w, r, http.StatusInternalServerError, "internal_error", "internal-error", "Internal Server Error", "Internal server error")
				return
			}
			operation := path.Operations()[r.Method]
			if operation.RequestBody == nil && r.Body != nil {
				var firstByte [1]byte
				if _, err := io.ReadFull(r.Body, firstByte[:]); err != io.EOF {
					handler.InvalidRequest(w, r)
					return
				}
			}
			input := &openapi3filter.RequestValidationInput{
				Request:    r,
				PathParams: map[string]string{"tripId": chi.URLParam(r, "tripId")},
				Route: &routers.Route{
					Spec: spec, Path: pattern, PathItem: path,
					Method: r.Method, Operation: operation,
				},
			}
			if err := openapi3filter.ValidateRequest(r.Context(), input); err != nil {
				handler.InvalidRequest(w, r)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
	return api.HandlerWithOptions(handler, api.ChiServerOptions{
		BaseRouter:  router,
		Middlewares: []api.MiddlewareFunc{validate},
		ErrorHandlerFunc: func(w http.ResponseWriter, r *http.Request, _ error) {
			handler.InvalidRequest(w, r)
		},
	}), nil
}
