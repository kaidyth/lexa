package routes

import (
	"net/http"

	"github.com/urfave/negroni"
)

// Route information
type Route struct {
	Pattern     string
	Methods     []string // Reference http.Method<X>
	Middlewares []negroni.Handler
}

// RouteElement represents the interface for all routes ([]interface{RouteElement} for arrya access)
type RouteElement interface {
	GetRoute() Route
	ServeHTTP(response http.ResponseWriter, request *http.Request)
}
