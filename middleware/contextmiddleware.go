package middleware

import (
	"context"
	"net/http"
)

type ContextMiddleware struct {
	context context.Context
}

// NewContextMiddleware creates a new global context that is passed to each request as part of the middleware chain
func NewContextMiddleware(ctx context.Context) *ContextMiddleware {
	return &ContextMiddleware{
		context: ctx,
	}
}

// ServeHTTP
func (m *ContextMiddleware) ServeHTTP(rw http.ResponseWriter, r *http.Request, next http.HandlerFunc) {
	next(rw, r.WithContext(m.context))
}
