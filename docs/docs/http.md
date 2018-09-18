---
title: HTTP Interface
---

# HTTP Interface

Mesos-DNS implements a simple REST API for service discovery over HTTP:

* `GET /v1/version`: lists the Mesos-DNS version
* `GET /v1/config`: lists the Mesos-DNS configuration info
* `GET /v1/hosts/{host}`: lists the IP address of a host
* `GET /v1/services/{service}`: lists the host, IP address, and port for a service
* `GET /v1/enumerate`: lists all DNS information

## `GET /v1/version`

Lists in JSON format the Mesos-DNS version and source code URL.

``` console
$ curl http://10.190.238.173:8123/v1/version
{
  "Service":"Mesos-DNS",
  "URL":"https://github.com/mesosphere/mesos-dns","Version":"0.1.1"
}
```

## `GET /v1/config`

Lists in JSON format the Mesos-DNS configuration parameters.

```console
curl http://10.190.238.173:8123/v1/config
{
  "Masters":null,
  "Zk":"zk://10.207.154.85:2181/mesos",
  "RefreshSeconds":60,
  "TTL":60,
  "Port":53,
  "Domain":"mesos",
  "Resolvers":["169.254.169.254","10.0.0.1"],
  "Timeout":5,
  "Email":"root.mesos-dns.mesos.",
  "Mname":"mesos-dns.mesos.",
  "Listener":"0.0.0.0",
  "HttpPort":8123,
  "DnsOn":true,
  "HttpOn":true
}
```

## `GET /v1/hosts/{host}`

Lists in JSON format the IP address(es) that correspond to a hostname. It is the equivalent of DNS A and AAAA record lookup.  Note, the HTTP interface only translates hostnames in the Mesos domain.

```console
$ curl http://10.190.238.173:8123/v1/hosts/nginx.marathon.mesos
[
  {"host":"nginx.marathon.mesos.","ip":"10.249.219.155"},
  {"host":"nginx.marathon.mesos.","ip":"10.190.238.173"},
  {"host":"nginx.marathon.mesos.","ip":"10.156.230.230"}
]
```

## `GET /v1/services/{service}`

Lists in JSON format the hostname, IP address, and ports that correspond to a hostname. It is the equivalent of DNS SRV record lookup.  Note, the HTTP interface only translates services in the Mesos domain.

```console
curl http://10.190.238.173:8123/v1/services/_nginx._tcp.marathon.mesos.
[
  {"host":"nginx-s2.marathon.mesos.","ip":"10.249.219.155","port":"31644", "service":"_nginx._tcp.marathon.mesos."},
  {"host":"nginx-s1.marathon.mesos.","ip":"10.190.238.173","port":"31667","service":"_nginx._tcp.marathon.mesos."},
  {"host":"nginx-s0.marathon.mesos.","ip":"10.156.230.230","port":"31880","service":"_nginx._tcp.marathon.mesos."}
]
```

## `GET /v1/enumerate`

Lists in JSON format all DNS information.

```console
curl http://127.0.0.1:8123/v1/enumerate
{
  "frameworks": [
    {
      "tasks": [],
      "name": "metronome"
    },
    {
      "tasks": [
        {
          "name": "nginx",
          "id": "nginx.48dccce7-90bc-11e6-ae70-70b3d5800001",
          "records": [
            {
              "name": "nginx.marathon.mesos.",
              "host": "10.10.0.93",
              "rtype": "A"
            },
            {
              "name": "nginx-jhy6r-s1.marathon.mesos.",
              "host": "10.10.0.93",
              "rtype": "A"
            },
            {
              "name": "nginx.marathon.slave.mesos.",
              "host": "10.10.0.93",
              "rtype": "A"
            },
            {
              "name": "nginx-jhy6r-s1.marathon.slave.mesos.",
              "host": "10.10.0.93",
              "rtype": "A"
            },
            {
              "name": "_nginx._tcp.marathon.slave.mesos.",
              "host": "nginx-jhy6r-s1.marathon.slave.mesos.:7564",
              "rtype": "SRV"
            },
            {
              "name": "_nginx._udp.marathon.slave.mesos.",
              "host": "nginx-jhy6r-s1.marathon.slave.mesos.:7564",
              "rtype": "SRV"
            },
            {
              "name": "_nginx._tcp.marathon.mesos.",
              "host": "nginx-jhy6r-s1.marathon.mesos.:7564",
              "rtype": "SRV"
            },
            {
              "name": "_nginx._nginx._tcp.marathon.mesos.",
              "host": "nginx-jhy6r-s1.marathon.mesos.:7564",
              "rtype": "SRV"
            }
          ]
        }
      ],
      "name": "marathon"
    }
  ]
}
```
