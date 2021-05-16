package routes

import (
	"net/http"
)

type RootRouteElement struct {
	Route Route
}

// NewRootRoute returns the routing data for /
func NewRootRoute() *RootRouteElement {
	rt := Route{
		Pattern: "/",
		Methods: []string{http.MethodGet}}

	return &RootRouteElement{
		Route: rt,
	}
}

// ServeHTTP runs the given action
func (r *RootRouteElement) ServeHTTP(response http.ResponseWriter, request *http.Request) {
	response.Header().Set("test", "test")
}

// GetRoute returns Route
func (r *RootRouteElement) GetRoute() Route {
	return r.Route
}
