package resolver

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io/ioutil"
	"math/rand"
	"os"
	"strings"
	"time"

	"github.com/apex/log"
	"github.com/kaidyth/lexa/server/dataset"
	"github.com/kaidyth/lexa/shared"
	"github.com/kaidyth/lexa/shared/messages"
	"github.com/knadh/koanf"
	"github.com/miekg/dns"
	"github.com/ryanuber/go-glob"
)

type ResolverServiceData struct {
	Service  messages.Service
	Host     dataset.Host
	Hostname string
}

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
	k := ctx.Value("koanf").(*koanf.Koanf)
	// Iterate over the question
	for _, q := range m.Question {
		// Grab the data source. This returns an error but []Hosts{} so we can ignroe the error
		ds, _ := dataset.NewDataset(ctx)

		// SRV Records aren't host specific, so pull the data directly from the dataset
		if q.Qtype == dns.TypeSRV {
			if dataset.IsServicesQuery(q.Name) {
				log.Trace(fmt.Sprintf("Query for %s %d", q.Name, q.Qtype))
				services, err := getAddressesForService(q.Name, ds)

				if err == nil {
					rand.Seed(time.Now().UnixNano())
					rand.Shuffle(len(services), func(i, j int) { services[i], services[j] = services[j], services[i] })
					for _, service := range services {
						interfaceBoundHostName, err := getInterfaceBoundHostNameForService(service)
						if err == nil {
							rr, err := dns.NewRR(fmt.Sprintf("%s 0 SRV %d %d %d %s", q.Name, 1, 1, service.Service.Port, interfaceBoundHostName+"."+k.String("service.suffix")))
							if err == nil {
								m.Answer = append(m.Answer, rr)
							}
						}
					}
				}
			}
		} else {
			hostname := dataset.GetBaseHostname(q.Name)
			log.Trace(fmt.Sprintf("Query for %s %d, Hostname: %s\n", q.Name, q.Qtype, hostname))

			// Filter the specific host data out
			var hosts []dataset.Host

			// Iterate over all hosts in the dataset, and glob match for wildcards, and construct an array of matching hosts
			for _, hostElement := range ds.Hosts {
				// Strip the .if., .interfaces., and .services. section from the query name
				if hostElement.Name+"." == hostname || glob.Glob(hostname, hostElement.Name+".") {
					hosts = append([]dataset.Host{hostElement}, hosts...)
				}
			}

			// Filter specific A, AAAA records
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
}

func getAddressesForService(queryString string, ds *dataset.Dataset) ([]ResolverServiceData, error) {
	var services []ResolverServiceData

	isRFC2782, serviceName, proto := isRFC2782(queryString)
	if isRFC2782 {
		// Iterate through all of the hosts and find all those that match the service name and protocol
		for _, host := range ds.Hosts {
			hasService, srv := dataset.HasService(host, serviceName)
			if hasService {
				// Return TCP/UDP first
				if (srv.Proto == "tcp" || srv.Proto == "udp") && srv.Proto == proto {
					service := ResolverServiceData{Service: srv, Host: host, Hostname: host.Name}
					services = append(services, service)
				} else {
					// Filter by tag otherwise
					if shared.Contains(srv.Tags, proto) {
						service := ResolverServiceData{Service: srv, Host: host, Hostname: host.Name}
						services = append(services, service)
					}
				}
			}
		}
	} else {
		tag, serviceName := getTagAndServiceName(queryString)
		for _, host := range ds.Hosts {
			hasService, srv := dataset.HasService(host, serviceName)
			if hasService {
				if tag == "" {
					service := ResolverServiceData{Service: srv, Host: host, Hostname: host.Name}
					services = append(services, service)
				} else {
					if shared.Contains(srv.Tags, tag) {
						service := ResolverServiceData{Service: srv, Host: host, Hostname: host.Name}
						services = append(services, service)
					}
				}
			}
		}
	}

	return services, nil
}

func getTagAndServiceName(queryString string) (string, string) {
	segments := strings.Split(queryString, ".")
	if len(segments) <= 3 {
		return "", ""
	}

	if segments[1] == "service" {
		return "", segments[0]
	}

	return segments[0], segments[1]
}

func isRFC2782(queryString string) (bool, string, string) {
	segments := strings.Split(queryString, ".")
	if len(segments) <= 4 {
		return false, "", ""
	}

	serviceName := segments[0]
	proto := segments[1]

	if serviceName[0:1] == "_" && proto[0:1] == "_" {
		return true, serviceName[1:len(serviceName)], proto[1:len(proto)]
	}

	return false, "", ""
}

func getInterfaceBoundHostNameForService(service ResolverServiceData) (string, error) {
	// If the service doesn't specify the interface to bind to, then push the first interface
	if service.Service.Interface == "" {
		return service.Host.Interfaces.IPv4[0].Name + ".if." + service.Host.Name, nil
	}

	// If a interface name _is_ specified, then we need to find that specific interface and return it
	for i, ifce := range service.Host.Interfaces.IPv4 {
		if ifce.Name == service.Service.Interface {
			return service.Host.Interfaces.IPv4[i].Name + ".if." + service.Host.Name, nil
		}
	}

	return "", errors.New("Interface not found")
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
