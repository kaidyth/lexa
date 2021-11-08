package mounts

import "github.com/kaidyth/lexa/server/api/routes"

// Mount definition
type Mount struct {
	MountPoint string
	Routes     []interface{ routes.RouteElement }
}
