package resolver

import (
	"context"
	"crypto/tls"
	"fmt"
	"io/ioutil"
	"os"

	"github.com/apex/log"
	"github.com/kaidyth/lexa/shared"
	"github.com/knadh/koanf"
	"github.com/miekg/dns"
)

// NewResolver creates a new DNS resolver
func NewResolver(ctx context.Context) *dns.Server {
	k := ctx.Value("koanf").(*koanf.Koanf)
	port := k.String("cluster.dns.port")
	bind := k.String("cluster.dns.bind")
	suffix := k.String("cluster.suffix")

	dns.HandleFunc(suffix+".", func(w dns.ResponseWriter, r *dns.Msg) {
		handleRequest(w, r, ctx)
	})

	server := &dns.Server{
		Addr: bind + ":" + port,
		Net:  "udp",
	}

	return server
}

func NewDoTResolver(ctx context.Context) *dns.Server {
	k := ctx.Value("koanf").(*koanf.Koanf)
	suffix := k.String("cluster.suffix")
	port := k.String("cluster.dns.tls.port")
	bind := k.String("cluster.dns.tls.bind")
	tlsKey := k.String("cluster.dns.tls.key")
	tlsCrt := k.String("cluster.dns.tls.certificate")

	dns.HandleFunc(suffix+".", func(w dns.ResponseWriter, r *dns.Msg) {
		handleRequest(w, r, ctx)
	})

	// If a TLS certificate and keyy aren't provided, generate one on demand
	if tlsKey == "" || tlsCrt == "" {
		log.Warn("Creating temporary self-signed DOT DNS certificate and key")
		kFile, err := ioutil.TempFile(os.TempDir(), "cluster.key")
		if err != nil {
			log.Fatal("Unable to create temporary file")
			os.Exit(1)
		}

		cFile, err := ioutil.TempFile(os.TempDir(), "cluster.crt")
		if err != nil {
			log.Fatal("Unable to create temporary file")
			os.Exit(1)
		}

		ECKey := shared.GenerateECKey(kFile)
		shared.GenerateCertificate(&ECKey.PublicKey, ECKey, cFile)

		tlsKey = kFile.Name()
		tlsCrt = cFile.Name()
		defer os.RemoveAll(cFile.Name())
		defer os.RemoveAll(kFile.Name())
	}

	cert, _ := tls.LoadX509KeyPair(tlsCrt, tlsKey)
	cfg := &tls.Config{
		MinVersion:               tls.VersionTLS12,
		CurvePreferences:         []tls.CurveID{tls.CurveP521, tls.CurveP384, tls.CurveP256},
		PreferServerCipherSuites: true,
		Certificates:             []tls.Certificate{cert},
		CipherSuites: []uint16{
			tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
			tls.TLS_ECDHE_RSA_WITH_AES_256_CBC_SHA,
			tls.TLS_RSA_WITH_AES_256_GCM_SHA384,
			tls.TLS_RSA_WITH_AES_256_CBC_SHA,
		},
	}

	server := &dns.Server{
		Addr:      bind + ":" + port,
		Net:       "tcp-tls",
		TLSConfig: cfg,
	}

	return server
}

func Shutdown(ctx context.Context, dnsServer *dns.Server) error {
	log.Trace("DNS Server shutdown")
	return dnsServer.ShutdownContext(ctx)
}

func StartServer(server *dns.Server) error {
	err := server.ListenAndServe()
	if err != nil {
		log.Fatal(fmt.Sprintf("Unable to start DNS server: %s", err))
	}

	return err
}

func parseQuery(m *dns.Msg, ctx context.Context) {
}

func handleRequest(w dns.ResponseWriter, r *dns.Msg, ctx context.Context) {
	m := new(dns.Msg)
	m.SetReply(r)
	m.Compress = false

	switch r.Opcode {
	case dns.OpcodeQuery:
		parseQuery(m, ctx)
	}

	w.WriteMsg(m)
}
