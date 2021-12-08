# Lexa - Instance and Service discovery for LXD Containers over DNS and REST

Lexa is a instance and service discovery tool for LXD containers that exposes DNS, DoT and a JSON REST endpoint to make discovery of containers and services within containers on LXD instances easier.

Lexa can be used as a stand-alone service to identify what LXD containers are running on a given host, or can be used in conjunction with tools such as HaProxy, Traefik, or even your own DNS server to automate load balancing between hosts, blue/green or red/black deployments, CNAME to hosts, and other scenariors. Lexa also provides a local agent (wip) that notifies the main server of any defines services running in the container. Think of it like a local, slimmed down Consul agent.

## Building

Install Golang 1.16+ then run the included `Makefile` to generate your own, or use one of the attached releases.

## Usage

### HTTPS JSON REST API

Lexa exposes a simple REST API for discoverying and listing hosts. This can be paired with another service to transform it to a more suitable format, such as a transformer for Traefik.

```
curl -qqsk https://127.0.0.1:18443/containers/ | jq .
{
  "hosts": [
    {
      "name": "nginx-php-74.lexa",
      "interfaces": {
        "ipv4": [
          {
            "eth0": "10.123.201.141"
          }
        ],
        "ipv6": [
          {
            "eth0": "fd42:aa04:d9d5:3541:216:3eff:fe06:6996"
          }
        ]
      },
      "services": [
        {
          "name": "http",
          "port": 80,
          "proto": "tcp",
          "tags": [
            "foo",
            "bar"
          ]
        },
        {
          "name": "https",
          "port": 443,
          "proto": "tcp",
          "tags": [
            "foo2",
            "bar2"
          ]
        }
      ]
    },
    {
      "name": "nginx-php-80.lexa",
      "interfaces": {
        "ipv4": [
          {
            "eth0": "10.123.201.178"
          }
        ],
        "ipv6": [
          {
            "eth0": "fd42:aa04:d9d5:3541:216:3eff:fe39:b94d"
          }
        ]
      },
      "services": null
    }
  ]
}
```

### DNS

Lexa will return the first listed interface in LXD (usually eth0). If you need a specific interface (such as a bridge), query by the interface name defined in the following section.

```
dig hostname.lexa @127.0.0.1 -p 18053
```

Both the DNS and DoT resolvers support wildcards, enabling you to query by a wildcard and be returned the appropriate hostname and IPs for all running containers that match.

Lexa supports both `A` IPv4 and `AAAA` IPv6 records.


```
dig nginx-php\*.lexa AAAA @127.0.0.1 -p 18053

; <<>> DiG 9.16.1-Ubuntu <<>> nginx-php*.lexa AAAA @127.0.0.1 -p 18053
;; global options: +cmd
;; Got answer:
;; ->>HEADER<<- opcode: QUERY, status: NOERROR, id: 51396
;; flags: qr rd; QUERY: 1, ANSWER: 2, AUTHORITY: 0, ADDITIONAL: 0
;; WARNING: recursion requested but not available

;; QUESTION SECTION:
;nginx-php*.lexa.        IN      AAAA

;; ANSWER SECTION:
nginx-php-80.lexa.      0       IN      AAAA    fd42:aa04:d9d5:3541:216:3eff:fe39:b94d
nginx-php-74.lexa.      0       IN      AAAA    fd42:aa04:d9d5:3541:216:3eff:fe06:6996

;; Query time: 35 msec
;; SERVER: 127.0.0.1#18053(127.0.0.1)
;; WHEN: Fri Sep 17 10:28:51 CDT 2021
;; MSG SIZE  rcvd: 138
```

#### Query a specific interface name

Specific interfaces can be queried via the `.interface.` or `.if.` for short prepend.

```
dig eth0.if.hostname.lexa @127.0.0.1 -p 18053
dig eth0.interface.hostname.lexa @127.0.0.1 -p 18053
```

#### Query a specific service name

Several options exists for querying services, and retrieving a list of IP's.

##### Standard Lookup

The format of the standard serviec lookup is:

```
[tag.].<service>.service.<prefix>
```

The tag parameter is option, and if provided will do result filtering. If no tag is provided, no filtering is done on the tag.

As an example, if we wanted to find all `https` servers, we could query `https.service.lexa`. If we're looking for our `primary` MySQL replica, utilizing tags we could perform tag filtering by querying: `primary.mysql.service.lexa`.

##### RFC 2782

The format for RFC 2782 SRV lookups is:

```
_<service>._<protocol>.service.<prefix>
```

The response of which will contain the following:

```
_<service>._<protocol>.service.<prefix> <ttl> IN SRV <priority> <weight> <port> <if_target>
```

> NOTE: The TTL will always be 0 to prevent caching. The `priority` and `weight` are currently not implemented, but may be added in a future version of Lexa.

SRV queries should use underscores, `_` as the prefix to the `service` and `protocol` values in the query to prevent DNS collisions. The `protocol` can either be `tcp`, `udp`, or any of the tags listed on the service. If `tcp` or `udp` is used, tag filtering will be disabled. If a service has no tags `tcp` or `udp` should be used.

Lexa will always return an interface specific query response. By default it will return the defualt interface for the container, unless specified otherwise in the agent `service` configuration for the given service. If a given interface does not exist in the agent config the service won't be returned.

For load balancing purposes, the order of responses are randomized on each query

```
dig _https._tcp.service.lexa @127.0.0.1 -p 18053

; <<>> DiG 9.16.1-Ubuntu <<>> _web._tcp.service.lexa @127.0.0.1 -p 18053 SRV
;; global options: +cmd
;; Got answer:
;; ->>HEADER<<- opcode: QUERY, status: NOERROR, id: 47260
;; flags: qr rd; QUERY: 1, ANSWER: 2, AUTHORITY: 0, ADDITIONAL: 0
;; WARNING: recursion requested but not available

;; QUESTION SECTION:
;_web._tcp.service.lexa.               IN       SRV

;; ANSWER SECTION:
_https._tcp.service.lexa. 0       IN     SRV      1 1 80 eth0.if.c1.lexa.
_https._tcp.service.lexa. 0       IN     SRV      1 1 80 eth0.if.c2.lexa.
```

### DoT

Lexa supports a DNS over TLS (DoT) resolver as well if your platform requires secure DNS between your resolvers. All DNS features are supported in the DoT resolver.

```
kdig hostname.lexa @127.0.0.1 -p 18853 +tls
```

## Configuration

Both Lexa server and Lexa agent are configured via a HCL file. Defaults are defined in common/config.go. Lexa monitors it's configuration file, and will automatically apply and reload the server on save.

### Lexa Server Sample Configuration

```hcl
server {
  # DNS Suffix
  suffix = "lexa"

  # LXD connection configuration
  lxd {
      # The LXD Socket connectiont o use
      # Priority is given to sockets over host port if both are defined.
      socket = "/var/snap/lxd/common/lxd/unix.socket"
  }

  # Rest HTTP API configuration
  tls {
      # The TLS port to use
      port = 18443

      # Whether or not use to use kernel so_resueport. Linux only
      so_resue_port = true

      # TLS Certificate and Key
      # If not provided, Lexa will generate a temporary one
      certificate = "/path/to/server.crt"
      key = "/path/to/server.key"

      # Whether or not to enable Mutual TLS and require a client certificate to
      mtls {
          ca_certificate = false
      }
  }

  # DNS Server configuration
  dns {
      # The DNS port to bind to
      port = 18053

      # DNS DoT sub-configuration
      tls {
          # The DoT port to bind to
          port = 18853

          # TLS Certificate and Key
          # If not provided, Lexa will generate a temporary one
          certificate = "/path/to/server.crt"
          key = "/path/to/server.key"
      }
  }

  p2p {
    # The address to bind to.
    # Lexa/Noise CANNOT bind to a loopback or multicast address, or 0.0.0.0
    # You must manually specify this.
    # With socket connections it is advised to set this to either the lxdbr0 interface, or the host IP
    bind = "192.168.64.1"

    # The Noise port to bind to
    port = 45861

    # The interval that peer discovery will run.
    # Note that this also sets a cache-timeout floor for data
    peerScanInterval = 5
  }
}

# Agent configuration
# This information is exlusively used for `lexa agent`
agent {
  p2p {
    # Defaults to the LXD container name, but may be overwritten manually
    hostname = "c1"

    # The address to bind to.
    # Lexa/Noise CANNOT bind to a loopback or multicast address, or 0.0.0.0
    # You must manually specify this.
    # With socket connections it is advised to set this to either the lxdbr0 interface, or the host IP
    bind = "192.168.64.1"

    # The Noise port to bind to
    port = 45861

    # The interval that peer discovery will run.
    # Note that this also sets a cache-timeout floor for data
    peerScanInterval = 5

    # An array of peers to bootstrap.
    # Nodes won't be able to connect unless at least one bootstrap node is specified
    # This usually will be the lxdbr0 gateway IP:<lexa_p2p_server_port>
    # Similarily to to the bind port, this must be the actual interface
    bootstrapPeers = [
      "192.168.64.1:45861"
    ]

    # The interval that peer discovery will run.
    # Note that this also sets a cache-timeout floor for data
    peerScanInterval = 5
  }

  # Each agent can host multiple services
  service {
        # The name to recognize the service by
        name = "http"

        # The port
        port = 80

        # The Protocol. TCP or UDP
        proto = "tcp"

        # Any custom tags you wish to assign to the service.
        # Lexa can aggregate tags for non RFC 2782 queries
        tags = [
            "app"         # app.http.service.lexa
        ]
        interface = "eth0" # Optional
    }

    service {
        name = "https"
        port = 443
        proto = "tcp"
        tags = [
            "app"         # app.https.service.lexa
        ]
    }
}
```

### Dynamic Interface Binding
All `bind` configuration keys support dynamic runtime configurations via [`hashicorp/go-sockaddr/template`](https://pkg.go.dev/github.com/hashicorp/go-sockaddr/template#section-readme), and will use the first address returned by the package as the bind ip address.

For instance where your container IP address may change but you need to bind it to a specific IP, the following may be used, as an example:

```
agent {
  p2p {
    bind = "{{ GetInterfaceIP \"enp0s2\" }}"
  }
}
```

When using methods that return string objects instead of single IP's, use `attr "address"` to filter just the address.

As an example:

```
server {
  dns {
    bind = "{{ GetPrivateInterfaces | attr \"address\" }}"
  }
}
```

Dynamic address binding is supported on all `bind` properties, and supports all features of the template package, however you must filter by only the address attribute to get a valid result.

## Commands

### Server

Starts a new DNS, DoT, and HTTPS REST server for instance and service discovery. The server should be configured to point to your LXD instance for discovery.

Usage: `lexa server --config /path/to/lexa.hcl`

> By default Lexa will search for lexa.hcl in the local and home directory of the user running it if a configuration path is not defined, and will gracefully fallback to sane defaults otherwise.

### Agent

A local agent that runs inside your LXD container to inform the server about services running on the host.

Usage: `lexa agent --config /path/to/lexa.hcl`

> By default Lexa will search for lexa.hcl in the local and home directory of the user running it if a configuration path is not defined, and will gracefully fallback to sane defaults otherwise.

### Cluster

Cluster provides an overlay interface for querying multiple lexa backends, which you can use to loadbalancer across multiple LXD servers. Lexa cluster simply queries all known backend `lexa server` nodes, then returns an aggregated dataset, and does not translate mixed IP addresses across different LXD networks or NATs.

As an example, you may use Lexa cluster if you have multiple servers running their own LXD instance, and need to know the bridge, or wireguard IP address of any node known to any backend Lexa server.

Lexa Cluster exposes the same HTTP and DNS API's as `lexa server`.

Usage: `lexa cluster --config /path/to/lexa.hcl`

> By default Lexa will search for lexa.hcl in the local and home directory of the user running it if a configuration path is not defined, and will gracefully fallback to sane defaults otherwise.

### Version

Usage: `lexa version`

Outputs the build version and build platform

## Integration Examples

### HaProxy

Use Lexa with HaProxy to provide high availability between multiple LXD containers, facilitate blue/green or red/black deployments via HaProxy DNS Discovery

```
global
    ssl-default-bind-ciphersuites TLS_AES_128_GCM_SHA256:TLS_AES_256_GCM_SHA384:TLS_CHACHA20_POLY1305_SHA256
    ssl-default-bind-options no-sslv3 no-tlsv10 no-tlsv11 no-tls-tickets

    tune.ssl.default-dh-param 4096
    user haproxy
    group haproxy
    maxconn  4000
    pidfile  /usr/local/var/run/haproxy.pid

defaults
    timeout client 15s
    timeout connect 15s
    timeout server 30s

resolvers lexa
    nameserver coredns 127.0.0.53:18053
    accepted_payload_size 8192

frontend https-in
    mode http
    option forwardfor
    option httplog
    option http-server-close
    option httpclose
    log global

    bind :443 tfo ssl crt /path/to/server.pem alpn h2,http/1.1
    bind    :80

    # Redirect non TLS Traffic to HTTPs
    http-request redirect location https://%[req.hdr(Host)]%[capture.req.uri] if !{ ssl_fc }

    default_backend             ha_lxd_service

backend ha_lxd_rfc2782
    mode http
    balance roundrobin
    server-template web 5 _https._tcp.service.lexa check resolvers lexa init-addr none  ssl verify none

backend ha_lxd_service
    mode http
    balance roundrobin
    server-template web 5 https.service.lexa check resolvers lexa init-addr none  ssl verify none

backend ha_lxd_service_tagged
    mode http
    balance roundrobin
    server-template web 5 app.https.service.lexa check resolvers lexa init-addr none  ssl verify none

# If your contains are deployed with a specific name schema:  eg example-<sha256:0:6>.lexa
# SRV Service discovery is preferred, but this is provided as an option if you have a fixed domain, or fixed domain schema you want to connect with
backend ha_lxd
    mode http
    balance roundrobin
    server-template web 5 example-*.lexa:443 check resolvers lexa init-addr none  ssl verify none
```

### CoreDNS

Integrate Lexa as a CoreDNS resolver so that you can have a single DNS resolver in your architecture to query against.

```
lexa {
    bind 127.0.0.53
    reload
    errors
    forward . 127.0.0.1:18053 {
            health_check 5s
    }

    # Or use the DoT resolver
    # forward . tls://127.0.0.1 {
    #     tls_servername <servername>
    #     health_check 5s
    # }
}
```

> On Ubuntu, it is recommended to configure systemd-resolvd to point to CoreDNS as your primary resolver.

## Limitations

Lexa does have some limitations to be aware of when using it.

### Configuration Hot Reloading

Experimental support is provided for hot-reloading configurations via `service.hotreload` and `agent.hotreload`. This behavior is experimental, and services may crash due to either invalid configuration entries on save. Setting these values to true will cause lexa to reload configuration on configuration file save.

### Agent Connectivity

Lexa utilizes `Noise` to achieve a rudimentary p2p connectivity between services. On a single machine ensure you utilize the `lxdbr0` or bridged IP's to ensure connectivity between all nodes.

### LXD Cluster

LXD clusters are currently untested. While the interface should work, no support is currently provided for clustered setups, and the results IP. Lexa currently cannot differentiate between ips across different hosts. When using Lexa cluster, ensure the bound addresses utilize either a bridged IP, or a shared VPN.