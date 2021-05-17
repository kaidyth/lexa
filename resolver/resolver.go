package resolver

import (
	"context"
	"crypto/tls"
	"fmt"
	"io/ioutil"
	"os"

	"github.com/apex/log"
	"github.com/kaidyth/lexa/common"
	"github.com/knadh/koanf"
	"github.com/miekg/dns"
)

var records = map[string]string{
	"test.lexa.": "192.168.0.2",
}

// NewResolver creates a new DNS resolver
func NewResolver(k *koanf.Koanf, ctx context.Context) *dns.Server {
	port := k.String("dns.port")
	suffix := k.String("suffix")

	dns.HandleFunc(suffix+".", func(w dns.ResponseWriter, r *dns.Msg) {
		handleRequest(w, r, ctx)
	})

	server := &dns.Server{
		Addr: ":" + port,
		Net:  "udp",
	}

	return server
}

func NewDoTResolver(k *koanf.Koanf, ctx context.Context) *dns.Server {
	suffix := k.String("suffix")
	port := k.String("dns.tls.port")
	tlsKey := k.String("dns.tls.key")
	tlsCrt := k.String("dns.tls.certificate")

	dns.HandleFunc(suffix+".", func(w dns.ResponseWriter, r *dns.Msg) {
		handleRequest(w, r, ctx)
	})

	// If a TLS certificate and keyy aren't provided, generate one on demand
	if tlsKey == "" || tlsCrt == "" {
		log.Warn("Creating temporary self-signed DNS certificate and key")
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
		Addr:      ":" + port,
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
	return server.ListenAndServe()
}

func parseQuery(m *dns.Msg, ctx context.Context) {
	for _, q := range m.Question {
		switch q.Qtype {
		case dns.TypeA:
			log.Trace(fmt.Sprintf("Query for %s\n", q.Name))
			ip := records[q.Name]
			if ip != "" {
				rr, err := dns.NewRR(fmt.Sprintf("%s A %s", q.Name, ip))
				if err == nil {
					m.Answer = append(m.Answer, rr)
				}
			}
		}
	}
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
