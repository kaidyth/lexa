package api

import (
	"context"
	"io/ioutil"
	"net"
	"net/http"

	"os"

	"crypto/tls"

	"github.com/apex/log"
	"github.com/gorilla/mux"
	"github.com/kaidyth/lexa/api/mounts"
	"github.com/kaidyth/lexa/common"
	"github.com/kaidyth/lexa/middleware"
	reuseport "github.com/kavu/go_reuseport"
	"github.com/knadh/koanf"
	"github.com/urfave/negroni"
)

// Returns a full route list
func getRouteList() []mounts.Mount {
	return []mounts.Mount{mounts.NewRootMount()}
}

// Configures the routers for the router
func configureRouter(context context.Context) *negroni.Negroni {
	// Create a new global router
	router := mux.NewRouter().StrictSlash(true)
	router.KeepContext = true

	// Definition of common middlewares that are re-used across all requests
	n := negroni.New()
	n.UseHandler(router)

	// Create a globally available context from the cobra cmd context that contains the configuration
	n.Use(middleware.NewContextMiddleware(context))

	// Iterate over all the available mountpoints and created a new sub-routed for each mountpoint
	for _, mount := range getRouteList() {
		subrouter := router.PathPrefix(mount.MountPoint).Subrouter().StrictSlash(true)

		// Iterate over each route within the mountpoint and mount it
		for _, r := range mount.Routes {
			rte := r.GetRoute()
			srneg := n

			// Mount any route specific middlewares
			for _, middleware := range rte.Middlewares {
				srneg.Use(middleware)
			}

			// Mount the final action
			subrouter.HandleFunc(rte.Pattern, func(rw http.ResponseWriter, req *http.Request) {
				r.ServeHTTP(rw, req)
			}).Methods(rte.Methods...)

			// Mount the mountpoint to the router
			router.PathPrefix(mount.MountPoint).Handler(srneg.With(
				negroni.Wrap(subrouter),
			))
		}
	}

	return n
}

// NewRouter defines a new router instance
func NewRouter(k *koanf.Koanf, ctx context.Context) *http.Server {
	router := configureRouter(ctx)
	port := k.String("tls.port")

	// Force a modern TLS configuration
	cfg := &tls.Config{
		MinVersion:               tls.VersionTLS12,
		CurvePreferences:         []tls.CurveID{tls.CurveP521, tls.CurveP384, tls.CurveP256},
		PreferServerCipherSuites: true,
		CipherSuites: []uint16{
			tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
			tls.TLS_ECDHE_RSA_WITH_AES_256_CBC_SHA,
			tls.TLS_RSA_WITH_AES_256_GCM_SHA384,
			tls.TLS_RSA_WITH_AES_256_CBC_SHA,
		},
	}

	server := &http.Server{
		Addr:         ":" + port,
		TLSConfig:    cfg,
		TLSNextProto: make(map[string]func(*http.Server, *tls.Conn, http.Handler), 0),
		Handler:      router,
		BaseContext:  func(_ net.Listener) context.Context { return ctx },
	}

	return server
}

func Shutdown(ctx context.Context, httpServer *http.Server) error {
	log.Trace("HTTP Server shutdown")
	return httpServer.Shutdown(ctx)
}

// StartServer starts a new HTTP Server
func StartServer(k *koanf.Koanf, server *http.Server) error {
	port := k.String("tls.port")
	tlsKey := k.String("tls.key")
	tlsCrt := k.String("tls.certificate")

	// If a TLS certificate and keyy aren't provided, generate one on demand
	if tlsKey == "" || tlsCrt == "" {
		log.Warn("Creating temporary self-signed certificate and key")
		kFile, err := ioutil.TempFile(os.TempDir(), "server.key")
		if err != nil {
			log.Fatal("Unable to create temporary file")
			os.Exit(1)
		}

		cFile, err := ioutil.TempFile(os.TempDir(), "server.crt")
		if err != nil {
			log.Fatal("Unable to create temporary file")
			os.Exit(1)
		}

		ECKey := common.GenerateECKey(kFile)
		common.GenerateCertificate(&ECKey.PublicKey, ECKey, cFile)

		tlsKey = kFile.Name()
		tlsCrt = cFile.Name()
		defer os.RemoveAll(cFile.Name())
		defer os.RemoveAll(kFile.Name())
	} else {
		if _, err := os.Stat(tlsKey); os.IsNotExist(err) {
			log.WithFields(log.Fields{
				"file": tlsKey,
			}).Fatal("Unable to access server key")
			os.Exit(1)
		}
		if _, err := os.Stat(tlsKey); os.IsNotExist(err) {
			log.WithFields(log.Fields{
				"file": tlsCrt,
			}).Fatal("Unable to access server certificate")
			os.Exit(1)
		}
	}

	// SO_REUSEPORT may be used to add support for multiple instances via systemd @%i
	if k.Bool("tls.so_reuse_port") {
		log.Trace("SO_REUSEPORT enabled")

		listener, _ := reuseport.Listen("tcp", ":"+port)

		defer listener.Close()
		return server.ServeTLS(listener, tlsCrt, tlsKey)
	}

	return server.ListenAndServeTLS(tlsCrt, tlsKey)
}
