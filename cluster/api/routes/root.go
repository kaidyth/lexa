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
	// ctx := request.Context()
	// Set a JSON response
	response.Header().Set("Content-Type", "application/json")

	response.Write([]byte("Cluster"))
}

// GetRoute returns Route
func (r *RootRouteElement) GetRoute() Route {
	return r.Route
}
