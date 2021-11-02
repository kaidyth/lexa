package resolver

import (
	"context"
	"crypto/tls"
	"fmt"
	"io/ioutil"
	"os"

	"github.com/apex/log"
	"github.com/kaidyth/lexa/server/dataset"
	"github.com/kaidyth/lexa/shared"
	"github.com/knadh/koanf"
	"github.com/miekg/dns"
	"github.com/ryanuber/go-glob"
)

// NewResolver creates a new DNS resolver
func NewResolver(ctx context.Context) *dns.Server {
	k := ctx.Value("koanf").(*koanf.Koanf)
	port := k.String("server.dns.port")
	bind := k.String("server.dns.bind")
	suffix := k.String("server.suffix")

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
	suffix := k.String("server.suffix")
	port := k.String("server.dns.tls.port")
	bind := k.String("server.dns.tls.bind")
	tlsKey := k.String("server.dns.tls.key")
	tlsCrt := k.String("server.dns.tls.certificate")

	dns.HandleFunc(suffix+".", func(w dns.ResponseWriter, r *dns.Msg) {
		handleRequest(w, r, ctx)
	})

	// If a TLS certificate and keyy aren't provided, generate one on demand
	if tlsKey == "" || tlsCrt == "" {
		log.Warn("Creating temporary self-signed DOT DNS certificate and key")
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
	// Iterate over the question
	for _, q := range m.Question {
		hostname := dataset.GetBaseHostname(q.Name)
		log.Trace(fmt.Sprintf("Query for %s %d, Hostname: %s\n", q.Name, q.Qtype, hostname))

		// Grab the data source. This returns an error but []Hosts{} so we can ignroe the erro
		ds, _ := dataset.NewDataset(ctx)

		// Filter the specific host data out
		var hosts []dataset.Host

		// Iterate over all hosts in the dataset, and glob match for wildcards, and construct an array of matching hosts
		for _, hostElement := range ds.Hosts {
			// Strip the .if., .interfaces., and .services. section from the query name
			if hostElement.Name+"." == hostname || glob.Glob(hostname, hostElement.Name+".") {
				hosts = append([]dataset.Host{hostElement}, hosts...)
			}
		}

		for _, host := range hosts {
			switch q.Qtype {
			case dns.TypeA:
				addresses, rt := getAddressesForQueryType(host, q.Name, "IPv4")

				for _, address := range addresses {
					rr, err := dns.NewRR(fmt.Sprintf("%s 0 %s %s", q.Name, rt, address.IP.String()))
					if err == nil {
						m.Answer = append(m.Answer, rr)
					}
				}
			case dns.TypeAAAA:
				addresses, rt := getAddressesForQueryType(host, q.Name, "IPv6")

				for _, address := range addresses {
					rr, err := dns.NewRR(fmt.Sprintf("%s 0 %s %s", q.Name, rt, address.IP.String()))
					if err == nil {
						m.Answer = append(m.Answer, rr)
					}
				}
			}
		}
	}
}

func getAddressesForQueryType(host dataset.Host, queryString string, t string) ([]dataset.InterfaceElement, string) {
	var addresses []dataset.InterfaceElement

	var r []dataset.InterfaceElement
	var rt string
	if t == "IPv4" {
		r = host.Interfaces.IPv4
		rt = "A"
	} else if t == "IPv6" {
		r = host.Interfaces.IPv6
		rt = "AAAA"
	}

	if !dataset.IsInterfaceQuery(queryString) && !dataset.IsServicesQuery(queryString) {
		if len(r) != 0 {
			addresses = append([]dataset.InterfaceElement{r[0]}, addresses...)
		}
	} else if dataset.IsInterfaceQuery(queryString) {
		interfaceName, err := dataset.GetInterfaceNameFromQuery(queryString)
		if err == nil {
			for _, addr := range r {
				if addr.Name == interfaceName {
					addresses = append([]dataset.InterfaceElement{addr}, addresses...)
				}
			}
		}
	}

	return addresses, rt
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
