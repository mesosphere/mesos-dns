Introduction to Mesos-DNS
=========

[Mesos-DNS](http://github.com/mesosphere/mesos-dns) supports service discovery in [Mesos](http://mesos.apache.org/) clusters. It allows applications running in the cluster to find each other through the domain name system ([DNS](http://en.wikipedia.org/wiki/Domain_Name_System)). Mesos-DNS is designed to be a minimal, stateless service that is easy to deploy and maintain.

The current version of Mesos-DNS is v0.1.0. It has been tested with Mesos version v0.21.0.   

## Architecture Overview

Mesos-DNS operates as shown in the diagram below:


![Image](./mesos-dns.png)

Mesos-DNS periodically contacts the Mesos master(s), retrieves the state of running all running tasks, and generates DNS records for these tasks (A and SRV records). As tasks start, finish, fail, or restart on the Mesos cluster, Mesos-DNS updates the DNS records to reflect the latest state. Tasks running on Mesos slaves can discover the IP addresses and ports of other tasks they depend upon by issuing DNS lookup requests. Mesos-DNS replies directly DNS requests for tasks launched by Mesos. For DNS requests for other hostnames or services, Mesos-DNS uses an external nameserver to derive replies.

Mesos-DNS is stateless. On a restart after a failure, it can retrieve the latest state from Mesos master(s) and serve DNS requests without further coordination. It can be easily replicated to improve availability or load balance DNS requests in clusters with large numbers of slaves. 

## Service Naming 

Mesos-DNS defines a DNS domain for Mesos tasks (default `.mesos`). Running tasks can be discovered by looking up A and, optionally, SRV records within the Mesos domain. 

A records associate hostnames to IP addresses. For task `<task>` launched by framework `<framework>`, Mesos-DNS generates an A record for hostname `<task>.<framework>.<domain>` that provides the IP address of the slave running the task. For example, other tasks can discover service `search` launch by the `marathon` framework with the following lookup:

> `$ dig search.marathon.mesos`<br>
> `; <<>> DiG 9.8.4-rpz2+rl005.12-P1 <<>> search.marathon.mesos`<br>
> `;; global options: +cmd`<br>
> `;; Got answer:`<br>
> `;; ->>HEADER<<- opcode: QUERY, status: NOERROR, id: 24471`<br>
> `;; flags: qr aa rd ra; QUERY: 1, ANSWER: 1, AUTHORITY: 1, ADDITIONAL: 0`<br>
> ` `<br>
> `;; QUESTION SECTION:`<br>
> `;task2.mesos.			IN	A`<br>
> ` `<br>
> `;; ANSWER SECTION:`<br>
> `search.marathon.mesos.		60	IN	A	10.9.87.94`<br>

**FIX** SRV records associate a service name to a hostname and an IP port.  For task `<task>` launched by framework `<framework>`, Mesos-DNS generates an SRV record for service name `<task>.<framework>.<._protocol>.<domain>`, where `protocol` is `udp` or `tcp`. For example, other tasks can discover service `search` launch by the `marathon` framework with the following lookup:

**FIX** > `$ dig search.marathon._tcp.mesos`<br>
**FIX** > `; <<>> DiG 9.8.4-rpz2+rl005.12-P1 <<>> search.marathon.mesos`<br>
**FIX** > `;; global options: +cmd`<br>
**FIX** > `;; Got answer:`<br>
**FIX** > `;; ->>HEADER<<- opcode: QUERY, status: NOERROR, id: 24471`<br>
**FIX** > `;; flags: qr aa rd ra; QUERY: 1, ANSWER: 1, AUTHORITY: 1, ADDITIONAL: 0`<br>
**FIX** > ` `<br>
**FIX** > `;; QUESTION SECTION:`<br>
**FIX** > `;task2.mesos.			IN	A`<br>
**FIX** > ` `<br>
**FIX** > `;; ANSWER SECTION:`<br>
**FIX** > `search.marathon.mesos.		60	IN	A	10.9.87.94`<br>

SRV records are generated only for tasks that have allocated a specific port through Mesos. 

## Features

Mesos-DNS provides the following features:
* Serves A and SRV records for running Mesos tasks. 
* Acts as a recursive DNS server for requests outside of the Mesos domain. 
* Automatically rotates between Mesos masters on master failures. 
* Continues serving existing DNS records even when Mesos master(s) are unreachable. 

## Installation & Configuration 

Refer to the separate [document](install.md) for installation, configuration, and maintenance instructions.  

## Performance

**FIX** performance per core on some platform

**FIX** recommednations for performance tuning
