package mounts

import "github.com/kaidyth/lexa/api/routes"

// NewRootMount returns all the routes under /
func NewRootMount() Mount {
	var r []interface{ routes.RouteElement }
	r = append(r, routes.NewRootRoute())

	return Mount{
		MountPoint: "/",
		Routes:     r}
}
