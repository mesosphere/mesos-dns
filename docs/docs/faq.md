---
title: Mesos-DNS FAQ
---

##  Frequently Asked Questions

---

#### Mesos-DNS fails to launch

Make sure that the port used for Mesos-DNS is available and not in use by another process. To use the recommended port `53`, you must start Mesos-DNS as root. 

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

#### Verbose mode

If you start Mesos-DNS in verbose mode using the `-v` argument, it  prints a variety of messages that are useful for debugging and performance tuning. 
