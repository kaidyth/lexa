package resolver

import (
	"context"
	"crypto/tls"
	"fmt"
	"io/ioutil"
	"os"
	"strings"

	"github.com/apex/log"
	"github.com/kaidyth/lexa/common"
	"github.com/knadh/koanf"
	"github.com/miekg/dns"
)

// NewResolver creates a new DNS resolver
func NewResolver(ctx context.Context) *dns.Server {
	k := ctx.Value("koanf").(*koanf.Koanf)
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

func NewDoTResolver(ctx context.Context) *dns.Server {
	k := ctx.Value("koanf").(*koanf.Koanf)
	suffix := k.String("suffix")
	port := k.String("dns.tls.port")
	tlsKey := k.String("dns.tls.key")
	tlsCrt := k.String("dns.tls.certificate")

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
	k := ctx.Value("koanf").(*koanf.Koanf)

	// Iterate over the question
	for _, q := range m.Question {

		// Grab the data source
		ds, err := common.NewDataset(k)
		if err != nil {
			return
		}

		// Filter the specific host data out
		var host common.Host
		interfaceName := ""
		serviceName := ""
		for _, hostElement := range ds.Hosts {
			// Search first for the exact hostname
			if hostElement.Name+"." == q.Name {
				host = hostElement
			} else if strings.Contains(q.Name, ".interfaces.") {
				// If the `.interfaces.` appears, then the user is searching for information for a specific interface
				asr := strings.Split(q.Name, ".")
				interfaceName = asr[0]

				fqdnWithoutInterfaces := strings.Replace(q.Name, interfaceName+".interfaces.", "", 1)
				if hostElement.Name+"." == fqdnWithoutInterfaces {
					host = hostElement
				}
			} else if strings.Contains(q.Name, ".intserviceserfaces.") {
				// If the `.interfaces.` appears, then the user is searching for information for a specific interface
				asr := strings.Split(q.Name, ".")
				serviceName = asr[0]

				fqdnWithoutInterfaces := strings.Replace(q.Name, serviceName+".services.", "", 1)
				if hostElement.Name+"." == fqdnWithoutInterfaces {
					host = hostElement
				}
			}
		}

		log.Trace(fmt.Sprintf("Query for %s %d\n", q.Name, q.Qtype))

		switch q.Qtype {
		case dns.TypeCNAME:
			if strings.Contains(q.Name, ".interfaces.") {
				fqdnWithoutInterfaces := strings.Replace(q.Name, interfaceName+".interfaces.", "", 1)
				// Only return a CNAME if the itnerface actually exists
				for _, hostElement := range host.Interfaces.IPv4 {
					if interfaceName == hostElement.Name {
						rr, err := dns.NewRR(fmt.Sprintf("%s 0 CNAME %s", q.Name, fqdnWithoutInterfaces))
						if err == nil {
							m.Answer = append(m.Answer, rr)
						}
					}
				}
			} else if strings.Contains(q.Name, ".services.") {
				//fqdnWithoutInterfaces := strings.Replace(q.Name, serviceName+".services.", "", 1)
				// Make sure the service exists

			}
		case dns.TypeA:
			var address common.InterfaceElement
			if interfaceName == "" && serviceName == "" {
				address = host.Interfaces.IPv4[0]
			} else if interfaceName != "" {
				for _, addresses := range host.Interfaces.IPv4 {
					if addresses.Name == interfaceName {
						address = addresses
						break
					}
				}
			} else if serviceName != "" {

			}

			rr, err := dns.NewRR(fmt.Sprintf("%s 0 A %s", q.Name, address.IP.String()))
			if err == nil {
				m.Answer = append(m.Answer, rr)
			}
		case dns.TypeAAAA:
			var address common.InterfaceElement
			if interfaceName == "" && serviceName == "" {
				address = host.Interfaces.IPv6[0]
			} else if interfaceName != "" {
				for _, addresses := range host.Interfaces.IPv6 {
					if addresses.Name == interfaceName {
						address = addresses
						break
					}
				}
			} else if serviceName != "" {

			}
			rr, err := dns.NewRR(fmt.Sprintf("%s 0 AAAA %s", q.Name, address.IP.String()))
			if err == nil {
				m.Answer = append(m.Answer, rr)
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
