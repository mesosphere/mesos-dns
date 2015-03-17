---
title: Mesos-DNS FAQ
---

##  Frequently Asked Questions & Troubleshooting

---

#### Verbose and very verbose modes

If you start Mesos-DNS in verbose mode using the `-v` or `-vv` arguments, it  prints a variety of messages that are useful for debugging and performance tuning. The `-vv` option will periodically print every A or SRV record Mesos-DNS generates. 

---


#### Mesos-DNS fails to launch

Make sure that the port used for Mesos-DNS is available and not in use by another process. To use the recommended port `53`, you must start Mesos-DNS as root. 

---

#### Slaves cannot connect to Mesos-DNS

Make sure that port `53` is not blocked by a firewall rule on your cluster. For example, [Google Cloud Platform](https://cloud.google.com/) blocks port `53` by default. 

Check the `/etc/resolv.conf` file. If multiple nameservers are listed and Mesos-DNS is not the first one, the slave will first connect to the other name servers. If `options rotate` is used and one of the listed nameservers is not Mesos-DNS, then you will get intermittent failures.

---

#### Mesos-DNS does not resolve names in the Mesos domain

Check the configuration file to make sure that Mesos-DNS is directed to the right master(s) for the Mesos cluster (`masters`). 
 
---

#### Mesos-DNS does not resolve names outside of the Mesos domain

Check the configuration file to make sure that Mesos-DNS is configured with the IP address of  external DNS servers (`resolvers`).

---

#### Updating the configuration file

When you update the configuration file, you need to restart Mesos-DNS. No state is lost on restart as Mesos-DNS is stateless and retrieves task state from the Mesos master(s). 

---

### DNS names are not user-friendly

Some frameworks register with longer, less user-friendly names. For example, earlier versions of marathon may register with names like `marathon-0.7.5`, which will lead to names like `search.marathon-0.7.5.mesos`. Make sure your framework registers with the desired name. For instance, you can launch marathon with ` --framework_name marathon` to get the framework registered as `marathon`.  

---

### Mesos-DNS is not communicating with the Mesos Master using the hosts configured in the 'masters' field

Mesos-DNS uses the `masters` field in the configuration file only for the initial requests to the Mesos master. The initial request for task state also return information about the current masters. This information is used for subsequent task state request. If you launch Mesos-DNS in verbose mode using `-v `, there will be a period stdout message that identifies which master Mesos-DNS is contacting at the moment. 

