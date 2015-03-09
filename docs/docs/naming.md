---
title: Service Naming
---

# Service Naming

Mesos-DNS defines a DNS domain for Mesos tasks (default `.mesos`, see [instructions on configruation](configuration-parameters.html)). Running tasks can be discovered by looking up A and, optionally, SRV records within the Mesos domain. 

## A Records

An A record associates a hostname to an IP address. For task `task` launched by framework `framework`, Mesos-DNS generates an A record for hostname `task.framework.domain` that provides the IP address of the specific slave running the task. For example, other Mesos tasks can discover the IP address for service `search` launched by the `marathon` framework with a lookup for `search.marathon.mesos`:

``` console
$ dig search.marathon.mesos

; <<>> DiG 9.8.4-rpz2+rl005.12-P1 <<>> search.marathon.mesos
;; global options: +cmd
;; Got answer:
;; ->>HEADER<<- opcode: QUERY, status: NOERROR, id: 24471
;; flags: qr aa rd ra; QUERY: 1, ANSWER: 1, AUTHORITY: 1, ADDITIONAL: 0

;; QUESTION SECTION:
;search.marathon.mesos.			IN	A

;; ANSWER SECTION:
search.marathon.mesos.		60	IN	A	10.9.87.94
```
 
## SRV Records

An SRV record associates a service name to a hostname and an IP port.  For task `task` launched by framework `framework`, Mesos-DNS generates an SRV record for service name `_task._protocol.framework.domain`, where `protocol` is `udp` or `tcp`. For example, other Mesos tasks can discover service `search` launched by the `marathon` framework with a lookup for lookup `_search._tcp.marathon.mesos`:

``` console
$ dig _search._tcp.marathon.mesos SRV

; <<>> DiG 9.8.4-rpz2+rl005.12-P1 <<>> _search._tcp.marathon.mesos SRV
;; global options: +cmd
;; Got answer:
;; ->>HEADER<<- opcode: QUERY, status: NOERROR, id: 33793
;; flags: qr aa rd ra; QUERY: 1, ANSWER: 1, AUTHORITY: 0, ADDITIONAL: 0

;; QUESTION SECTION:
;_search._tcp.marathon.mesos.	IN SRV

;; ANSWER SECTION:
_search._tcp.marathon.mesos.	60 IN SRV 0 0 31302 10.254.132.41.
``` 

SRV records are generated only for tasks that have been allocated a specific port through Mesos. 


## Notes

If a framework launches multiple tasks with the same name, the DNS lookup will return multiple records, one per task. Mesos-DNS randomly shuffles the order of records to provide rudimentary load balancing between these tasks. 

Mesos-DNS does not support other types of DNS records at this point, including the PTR records needed for reverse lookups. DNS requests for records of type`ANY`, `A`, or `SRV` will return any A or SRV records found. DNS requests for records of other types in the Mesos domain will return `NXDOMAIN`.

Some frameworks register with longer, less friendly names. For example, earlier versions of marathon may register with names like `marathon-0.7.5`, which will lead to names like `search.marathon-0.7.5.mesos`. Make sure your framework registers with the desired name. For instance, you can launch marathon with ` --framework_name marathon` to get the framework registered as `marathon`.  

## Special Records

Mesos-DNS generates a few special records. Specifically, it creates A records (`master.domain`) and SRV records (`_master._tcp.domain` and `_master._udp.domain`) for every Mesos master in the cluster. There is set of records for the leading master (A record for `leader.domain` and SRV records for `_leader._tcp.domain` and `_leader._udp.domain`). Note that Mesos-DNS discovers the leading master when it regenerates DNS records. Hence, the records for the leader will not be updated instantaneously when new leader is elected. Finally Mesos-DNS generates A records for itself (`mesos-dns.domain`) that list all the IP addresses that Mesos-DNS is listening to. 

