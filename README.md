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
      "services": null
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

Specific services exposed by the agent can be queried

```
dig nginx.service.hostname.lexa @127.0.0.1 -p 18053
```

### DoT

Lexa supports a DNS over TLS (DoT) resolver as well if your platform requires secure DNS between your resolvers:

```
kdig hostname.lexa @127.0.0.1 -p 18853 +tls
```

## Configuration

Both Lexa server and Lexa agent are configured via a HCL file. Defaults are defined in common/config.go. Lexa monitors it's configuration file, and will automatically apply and reload the server on save.

### Lexa Server Sample Configuration

```hcl
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

# IPFS Server information
ipfs {
    # IPFS address to bind to
    bind = "0.0.0.0"

    # IPFS port to bind to
    port = 9000
}

# Agent configuration
# This information is exlusively used for `lexa agent`
agent {
    # An array of IPFS peers the agent should bootstrap to
    # At minimum, this should be your Lexa host or Lexa relay
    peers = [
        "127.0.0.1"
    ]
}
```

## Commands

### Server

Starts a new DNS, DoT, and HTTPS REST server for instance and service discovery. The server should be configured to point to your LXD instance for discovery.

Usage: `lexa server --config /path/to/lexa.hcl`

> By default Lexa will search for lexa.hcl in the local and home directory of the user running it if a configuration path is not defined, and will gracefully fallback to sane defaults otherwise.

### Agent

A local agent that runs inside your LXD container to inform the server about services running on the host.

Usage: `lexa agent --config /path/to/lexa.hcl`

> By default Lexa will search for lexa.hcl in the local and home directory of the user running it if a configuration path is not defined, and will gracefully fallback to sane defaults otherwise.

### Relay

The relay server provides a bridge between the lexa agent and lexa hosts if they aren't able to communicate directly.

Usage: `lexa relay --config /path/to/lexa.hcl`

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

    default_backend             ha_lxc

backend ha_lxd
    mode http
    balance roundrobin
    server-template web 5 example-*.lexa:443 check resolvers lexa init-addr none  ssl verify none
```

### CoreDNS

Integrate Lexa as a CoreDNS upstream

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