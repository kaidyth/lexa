package common

import (
	"encoding/json"
	"errors"
	"log"

	"github.com/knadh/koanf"
	lxd "github.com/lxc/lxd/client"
	"inet.af/netaddr"
)

type InterfaceElement struct {
	Name string     `json:"name"`
	IP   netaddr.IP `json:"address"`
}

type Interfaces struct {
	IPv4 []InterfaceElement `json:"ipv4"`
	IPv6 []InterfaceElement `json:"ipv6"`
}

type Host struct {
	Name       string     `json:"name"`
	Interfaces Interfaces `json:"interfaces"`
	Services   []string   `json:"services"` // @TODO Add Service Support
}

type Dataset struct {
	Hosts []Host `json:"hosts"`
}

func (i Interfaces) MarshalJSON() ([]byte, error) {
	// Create a Mock interface mapping
	type InterfacesMap struct {
		IPv4 []map[string]string `json:"ipv4"`
		IPv6 []map[string]string `json:"ipv6"`
	}

	interfaceMap := InterfacesMap{}

	for _, ifm := range i.IPv4 {
		s := make(map[string]string)
		s[ifm.Name] = ifm.IP.String()
		interfaceMap.IPv4 = append(interfaceMap.IPv4, s)
	}

	for _, ifm := range i.IPv6 {
		s := make(map[string]string)
		s[ifm.Name] = ifm.IP.String()
		interfaceMap.IPv6 = append(interfaceMap.IPv6, s)
	}
	return json.Marshal(interfaceMap)
}

func NewDataset(k *koanf.Koanf) (*Dataset, error) {
	hosts, err := initHosts(k)
	if err != nil {
		hosts := []Host{}
		log.Fatal("Unable to fetch hosts from upstream")
		return &Dataset{
			Hosts: hosts}, errors.New("Unable to fetch hosts")
	}

	return &Dataset{
		Hosts: hosts}, nil
}

func getConnection(k *koanf.Koanf) (lxd.InstanceServer, error) {
	if k.String("lxd.socket") != "" {
		instance, err := lxd.ConnectLXDUnix(k.String("lxd.socket"), nil)
		if err != nil {
			return nil, err
		}

		return instance, nil
	}

	// else setup the http connection
	return nil, nil
}

func initHosts(k *koanf.Koanf) ([]Host, error) {
	conn, err := getConnection(k)
	if err != nil {
		return nil, err
	}

	// Get the Full Container Details
	containers, err := conn.GetContainersFull()
	if err != nil {
		return nil, err
	}

	hosts := []Host{}

	// Iterate over all of the containers to get their network information
	for _, container := range containers {
		// Only pull data from running containers
		// Ref: https://lxd.readthedocs.io/en/latest/rest-api/#list-of-current-status-codes
		if container.State.StatusCode != 103 {
			continue
		}

		host := Host{Name: container.Name + "." + k.String("suffix")}
		interfaces := Interfaces{
			IPv4: []InterfaceElement{},
			IPv6: []InterfaceElement{}}

		// Iterate over each network associate to the container
		for networkName, network := range container.State.Network {
			interfaceElement := InterfaceElement{Name: networkName}

			// Iterate over each address to get the IP address information for the given interface
			for _, address := range network.Addresses {
				ip, err := netaddr.ParseIP(address.Address)
				if err == nil {
					// Ignore multicast and unicast networks
					if ip.IsLoopback() || ip.IsMulticast() || ip.IsInterfaceLocalMulticast() || ip.IsLinkLocalMulticast() || ip.IsLinkLocalUnicast() {
						continue
					}

					interfaceElement.IP = ip

					// Prepend the elements to the correct list
					if ip.Is4() {
						interfaces.IPv4 = append([]InterfaceElement{interfaceElement}, interfaces.IPv4...)
					} else if ip.Is6() {
						interfaces.IPv6 = append([]InterfaceElement{interfaceElement}, interfaces.IPv6...)
					}
				}
			}

			// We also need to discover all other interfaces on the container
		}

		host.Interfaces = interfaces
		hosts = append(hosts, host)
	}

	return hosts, nil
}
