# Lexa - Instance and Service discovery for LXD Containers over DNS and REST

Lexa is a instance and service discovery tool for LXD containers that exposes DNS, DoT and a JSON REST endpoint to make discovery of containers and services within containers on LXD instances easier.

Lexa can be used as a stand-alone service to identify what LXD containers are running on a given host, or can be used in conjunction with tools such as HaProxy, Traefik, or even your own DNS server to automate load balancing between hosts, blue/green or red/black deployments, CNAME to hosts, and other scenariors. Lexa also provides a local agent (wip) that notifies the main server of any defines services running in the container. Think of it like a local, slimmed down Consul agent.

## Building

Lexa is built using `Rust`, and can be built by running `cargo build`

## 0.2.0

Lexa 0.2 is a complete rewrite of Lexa in rust to take advantage of enhanced performance and memory related issues with the Go variant. While identical in functionality, there are some minor configuration changes from the 0.1 variant, chiefly among the configuration.

The main configuration difference is that communication to the LXD client is now done over HTTP, rather than the unix-socket. Consequently, to connect Lexa to LXD you'll need to expose the TLS API, and add a trusted certificate.

The necessary configuration for LXD is as follows:

```bash
# Expose the TLS API
$ lxc config set core.https_address [::]:8443
# Generate a certificate
$ openssl ecparam -genkey -name prime256v1 -out lxd.key
$ openssl req -x509 -new -SHA384 -nodes -key lxd.key -days 36500 -out lxd.crt
# Add the trust certificate to LXD
$  lxc config trust add lxd.crt
```

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
[tag].<service>.service.<prefix>
```

The tag parameter is option, and if provided will do result filtering. If no tag is provided, no filtering is done on the tag.

As an example, if we're looking for our `primary` MySQL replica, utilizing tags we could perform tag filtering by querying: `primary.mysql.service.lexa`.

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

Lexa supports a DNS over TLS (DoT) resolver and all DNS features are supported by this resolver.

```
kdig hostname.lexa @127.0.0.1 -p 18853 +tls
```

### DoH

Lexa supports DNS over HTTPS (DoH) resolver and all DNS features are supported by this resolver.

### DoQ

Lexa supports DNS over Quic (QoQ) resolver, and all DNS features are supported by this resolver.

## Configuration

Both Lexa server and Lexa agent are configured via a HCL file.
### Lexa Server Sample Configuration

```hcl
server {
  lxd {
      suffix = "lexa"
      bind {
        port = 8443
        host = "192.168.64.6"
      }
      certificate = "server.crt"
      key = "pkcs8.key"
  }

  # Rest HTTP API configuration
  tls {
      # The TLS port to use
      bind {
          port = 18443
          host = "0.0.0.0"
        }

      # Whether or not use to use kernel so_resueport. Linux only
      so_resue_port = true

      # TLS Certificate and Key
      # If not provided, Lexa will generate a temporary one
      certificate = "server.crt"
      key = "pkcs8.key"
  }

  # DNS Server configuration
  dns {
      # The DNS port to bind to
     bind {
          port = 18053
          host = "0.0.0.0"
        }

      quic {
          bind {
            port = 18854
            host = "0.0.0.0"
          }
          certificate = "server.crt"
           key = "pkcs8.key"
          hostname = "lexa.kaidyth.com"
      }

      # DNS DoT sub-configuration
      dot {
         bind {
          port = 18854
          host = "0.0.0.0"
        }
          certificate = "server.crt"
          key = "pkcs8.key"
      }

      # DNS DoH sub-configuration
      doh {
        bind {
          port = 18855
          host = "0.0.0.0"
        }

          certificate = "server.crt"
          key = "pkcs8.key"
          hostname = "lexa.kaidyth.com"
      }
  }

  log {
    out = "stdout"
    level = "info"
  }
}
```

## Commands

### Server

Starts a new DNS, DoT, and HTTPS REST server for instance and service discovery. The server should be configured to point to your LXD instance for discovery.

Usage: `lexa server --config /path/to/lexa.hcl`

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

## LXD Cluster

Lexa fully supports LXD Clusters using the `cluster.<instance>.lexa` query type for A or AAAA records, which will return the external IP of the LXD host the instance is running on.

```
# dig cluster.instance*.lexa CNAME

; <<>> DiG 9.18.1-1ubuntu1.2-Ubuntu <<>> cluster.instance*.lexa CNAME
;; global options: +cmd
;; Got answer:
;; ->>HEADER<<- opcode: QUERY, status: NOERROR, id: 40733
;; flags: qr aa rd; QUERY: 1, ANSWER: 1, AUTHORITY: 0, ADDITIONAL: 0
;; WARNING: recursion requested but not available

;; QUESTION SECTION:
;cluster.instance*.lexa.		IN	CNAME

;; ANSWER SECTION:
cluster.instance*.lexa.	3	IN	CNAME node-1.

;; Query time: 3 msec
;; SERVER: 127.0.0.1#53 (UDP)
;; WHEN: Fri Dec 30 22:15:48 UTC 2022
;; MSG SIZE  rcvd: 75
```

For querying to work, you _MUST_ have the `location` property on each LXD node be resolvable via DNS, either by hostname, or upstream DNS. It is advised to use a tool such as a Consul to have the hostname be resolvable, and then chain with Bind9, Unbound, or CoreDNS.

Once you have the DNS record of the host, you can query the instance A or AAAA record on the specific node.

```
# dig node-1.cluster.instance*.lexa A

; <<>> DiG 9.18.1-1ubuntu1.2-Ubuntu <<>> node-1.cluster.instance*.lexa CNAME
;; global options: +cmd
;; Got answer:
;; ->>HEADER<<- opcode: QUERY, status: NOERROR, id: 40733
;; flags: qr aa rd; QUERY: 1, ANSWER: 1, AUTHORITY: 0, ADDITIONAL: 0
;; WARNING: recursion requested but not available

;; QUESTION SECTION:
;code-1.cluster.instance*.lexa.		IN	A

;; ANSWER SECTION:
node-1.cluster.instance*.lexa.	3	IN	A 10.0.2.1

;; Query time: 3 msec
;; SERVER: 127.0.0.1#53 (UDP)
;; WHEN: Fri Dec 30 22:15:49 UTC 2022
;; MSG SIZE  rcvd: 75
```