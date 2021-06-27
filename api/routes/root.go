package routes

import (
	"encoding/json"
	"net/http"

	"github.com/kaidyth/lexa/common"
	"github.com/knadh/koanf"
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
	ctx := request.Context()
	k := ctx.Value("koanf").(*koanf.Koanf)

	// Set a JSON response
	response.Header().Set("Content-Type", "application/json")

	ds, err := common.NewDataset(k)
	if err != nil {
		response.WriteHeader(http.StatusInternalServerError)
		return
	}

	jsonDs, err := json.Marshal(ds)
	if err != nil {
		response.WriteHeader(http.StatusInternalServerError)
		return
	}

	response.Write(jsonDs)
}

// GetRoute returns Route
func (r *RootRouteElement) GetRoute() Route {
	return r.Route
}
