---
title: Service Naming
---

# Service Naming

Mesos-DNS defines a DNS domain for Mesos tasks (default `.mesos`, see [instructions on configruation](configuration-parameters.html)). Running tasks can be discovered by looking up A and, optionally, SRV records within the Mesos domain. 

## A Records

An A record associates a hostname to an IP address. For task `task` launched by framework `framework`, Mesos-DNS generates an A record for hostname `task.framework.domain` that provides the IP address of the specific slave running the task. For example, other Mesos tasks can discover the IP address for service `nginx` launched by the `marathon` framework with a lookup for `nginx`:

``` console
 dig nginx.marathon.mesos

; <<>> DiG 9.8.4-rpz2+rl005.12-P1 <<>> nginx.marathon.mesos
;; global options: +cmd
;; Got answer:
;; ->>HEADER<<- opcode: QUERY, status: NOERROR, id: 59991
;; flags: qr aa rd ra; QUERY: 1, ANSWER: 1, AUTHORITY: 0, ADDITIONAL: 0

;; QUESTION SECTION:
;nginx.marathon.mesos.		IN	A

;; ANSWER SECTION:
nginx.marathon.mesos.	60	IN	A	10.190.238.173
```
 
## SRV Records

An SRV record associates a service name to a hostname and an IP port.  For task `task` launched by framework `framework`, Mesos-DNS generates an SRV record for service name `_task._protocol.framework.domain`, where `protocol` is `udp` or `tcp`. For example, other Mesos tasks can discover service `nginx` launched by the `marathon` framework with a lookup for lookup `_nginx._tcp.marathon.mesos`:

``` console
$ dig SRV _nginx._tcp.marathon.mesos

; <<>> DiG 9.8.4-rpz2+rl005.12-P1 <<>> SRV _nginx._tcp.marathon.mesos
;; global options: +cmd
;; Got answer:
;; ->>HEADER<<- opcode: QUERY, status: NOERROR, id: 42765
;; flags: qr aa rd ra; QUERY: 1, ANSWER: 1, AUTHORITY: 0, ADDITIONAL: 1

;; QUESTION SECTION:
;_nginx._tcp.marathon.mesos.	IN	SRV

;; ANSWER SECTION:
_nginx._tcp.marathon.mesos. 60	IN	SRV	0 0 31667 nginx-s1.marathon.mesos.

;; ADDITIONAL SECTION:
nginx-s1.marathon.mesos. 60	IN	A	10.190.238.173
``` 

The SRV record for `_nginx._tcp.marathon.mesos` points to port 31667 on hostname `nginx-s1.marathon.mesos.`. The hostname corresponds to task `nginx` running on slave `s0`. The IP address for this hostname is provided in the additional section of the SRV reply, but can also be accessed through additional DNS requests for A records. The indirection using per slave hostnames is separate identically named tasks that run on different slaves and use different port numbers. 

SRV records are generated only for tasks that have been allocated specific ports through Mesos. 

## Notes

If a framework launches multiple tasks with the same name, the DNS lookup will return multiple records, one per task. Mesos-DNS randomly shuffles the order of records to provide rudimentary load balancing between these tasks. 

Mesos-DNS does not support other types of DNS records at this point (PTR, TXT, etc). DNS requests for records of type`ANY`, `A`, or `SRV` will return any A or SRV records found. DNS requests for records of other types in the Mesos domain will return `NXDOMAIN`.

Some frameworks register with longer, less friendly names. For example, earlier versions of marathon may register with names like `marathon-0.7.5`, which will lead to names like `search.marathon-0.7.5.mesos`. Make sure your framework registers with the desired name. For instance, you can launch marathon with ` --framework_name marathon` to get the framework registered as `marathon`.  

## Special Records

Mesos-DNS generates a few special records. Specifically, it creates a set of records for the leading master (A record for `leader.domain` and SRV records for `_leader._tcp.domain` and `_leader._udp.domain`). It also creates creates A records (`master.domain`) for every Mesos master it knows about. Note that, if you configure Mesos-DNS to detect the leading master through Zookeeper, then this is the only master it knows about. If you configure Mesos-DNS using the `masters` field, it will generate master records for every master in the list. Also not that the is inherent delay between the election of a new master and the update of leader/master records in Mesos-DNS. Finally Mesos-DNS generates A records for itself (`mesos-dns.domain`) that list all the IP addresses that Mesos-DNS is listening to. 

