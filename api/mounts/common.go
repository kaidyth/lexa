package mounts

import "github.com/kaidyth/lexa/api/routes"

// Mount definition
type Mount struct {
	MountPoint string
	Routes     []interface{ routes.RouteElement }
}
