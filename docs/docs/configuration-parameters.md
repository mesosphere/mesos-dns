---
title: Mesos-DNS Configuration Parameters
---

##  Mesos-DNS Configuration Parameters

Mesos-DNS is configured through the parameters in a json file. You can point Mesos-DNS to a specific configuration file using the argument `-config=pathto/file.json`. If no configuration file is passed as an argument, Mesos-DNS will look for file `config.json` in the current directory. 

The configuration file should include the following fields:

```
{
  "masters": ["10.101.160.15:5050", "10.101.160.16:5050", "10.101.160.17:5050"],
  "refreshSeconds": 60,
  "ttl": 60,
  "domain": "mesos",
  "port": 53,
  "resolvers": ["169.254.169.254"],
  "timeout": 5, 
  "listener": "10.101.160.16",
  "email": "root.mesos-dns.mesos"
}
```


`masters` is a comma separated list with the IP address and port number for the master(s) in the Mesos cluster. Mesos-DNS will automatically find the leading master at any point in order to retrieve state about running tasks. If there is no leading master or the leading master is not responsive, Mesos-DNS will continue serving DNS requests based on stale information about running tasks. The `masters` field is required. 

`refreshSeconds` is the frequency at which Mesos-DNS updates DNS records based on information retrieved from the Mesos master. The default value is 60 seconds. 

`ttl` is the [time to live](http://en.wikipedia.org/wiki/Time_to_live#DNS_records) value for DNS records served by Mesos-DNS, in seconds. It allows caching of the DNS record for a period of time in order to reduce DNS request rate. `ttl` should be equal or larger than `refreshSeconds`. The default value is 60 seconds. 

`domain` is the domain name for the Mesos cluster. The domain name can use characters [a-z, A-Z, 0-9], `-` if it is not the first or last character of a domain portion, and `.` as a separator of the textual portions of the domain name. We recommend you avoid valid [top-level domain names](http://en.wikipedia.org/wiki/List_of_Internet_top-level_domains). The default value is `mesos`.

`port` is the port number that Mesos-DNS monitors for incoming DNS requests from slaves. Requests can be sent over TCP or UDP. We recommend you use port `53` as several applications assume that the DNS server listens to this port. The default value is `53`.

`resolvers` is a comma separated list with the IP addresses of external DNS servers that Mesos-DNS will contact to resolve any DNS requests outside the `domain`. We ***recommend*** that you list the nameservers specified in the `/etc/resolv.conf` on the server Mesos-DNS is running. Alternatively, you can list `8.8.8.8`, which is the [Google public DNS](https://developers.google.com/speed/public-dns/) address. The `resolvers` field is required. 
 
`timeout` is the timeout threshold, in seconds, for connections and requests to external DNS requests. The default value is 5 seconds. 

`listener` is the IP address of Mesos-DNS. In SOA replies, Mesos-DNS identifies hostname `mesos-dns.domain` as the primary nameserver for the domain. It uses this IP address in an A record for `mesos-dns.domain`. The default value is "0.0.0.0", which instructs Mesos-DNS to create an A record for every IP address associated with a network interface on the server that runs the Mesos-DNS process. 

`email` is the email address of the Mesos domain name administrator. It is associated with the SOA record for the Mesos domain. The format is `mailbox-name.domain`, using a `.` instead of `@`. For example, if the email address is `root@mesos-dns.mesos`, the `email` field should be `root.mesos-dns.mesos`. The default value is `root.mesos-dns.mesos`.
