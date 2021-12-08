package shared

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/apex/log"
	template "github.com/hashicorp/go-sockaddr/template"
	"github.com/knadh/koanf"
	"inet.af/netaddr"
)

func Find(slice []string, val string) (int, bool) {
	for i, item := range slice {
		if item == val {
			return i, true
		}
	}
	return -1, false
}

func Contains(s []string, str string) bool {
	for _, v := range s {
		if v == str {
			return true
		}
	}
	return false
}

// GetNetworkBindings returns a netaddr.IP, port, and/or and error
func GetNetworkBindings(ctx context.Context, configKey string) (netaddr.IP, uint16, error) {
	k := ctx.Value("koanf").(*koanf.Koanf)
	bind := k.String(configKey + ".bind")
	port := uint16(k.Int(configKey + ".port"))
	ip, err := netaddr.ParseIP(bind)

	if err != nil {
		results, err := template.Parse(bind)

		if err != nil {
			log.Fatal(fmt.Sprintf("Unable to get bind from config file or template. Unable to proceed. %v", bind))
		}

		var stringIps []string = strings.Split(results, " ")
		if len(stringIps) <= 0 {
			log.Fatal(fmt.Sprintf("Unable to get bind from config file or template. Unable to proceed."))
			return netaddr.IPv4(0, 0, 0, 0), 0, errors.New(fmt.Sprintf("Unparsable IP template %s", configKey))
		}

		var ips []netaddr.IP
		for _, ip := range stringIps {
			d, err := netaddr.ParseIP(ip)
			if err == nil {
				ips = append(ips, d)
			}
		}

		if len(ips) <= 0 {
			log.Fatal(fmt.Sprintf("Unable to get bind from config file or template. Unable to proceed."))
			return netaddr.IPv4(0, 0, 0, 0), 0, errors.New(fmt.Sprintf("%s returned 0 addresses", configKey))
		}

		ip = ips[0]
	}

	return ip, port, nil

}
