---
title: Mesos-DNS on GCP
---

## Mesos-DNS on Google Compute Platform

This is a step-by-step tutorial for running Mesos-DNS with [Mesosphere](https://google.mesosphere.com/) on [Google Compute Platform](https://cloud.google.com). 

### Step 1: Launch a Mesosphere cluster

Launch a development Mesosphere cluster following the instructions at [https://google.mesosphere.com/](https://google.mesosphere.com/). Note that you will need a [Google Cloud Platform](https://cloud.google.com) account with an active project that has billing enabled.

The development cluster includes 1 master and 3 slaves.  We will use the information on the example cluster listed below for this tutorial. Note that we will use the internal IP addresses for all nodes in all cases. The cluster runs Mesos (version 0.21.1) and Marathon (version 0.7.6).

<p class="text-center">
  <img src="{{ site.baseurl}}/img/example-cluster-gce.png" width="650"  alt="">
</p>

[Google Cloud Platform](https://cloud.google.com) blocks port `53` by default. To unblock traffic for port `53` follow these [directions](http://stackoverflow.com/questions/21065922/how-to-open-a-specific-port-such-as-9090-in-google-compute-engine). Note that you need to unblock port `53` for both `tcp` and `udp` traffic. At the end, if you examine the firewall rules for your cluster you have a rule such as the one listed below, which opens port `53` for `udp` and `tcp` traffic from hosts in the `10.0.0.0` subnet. 

<p class="text-center">
  <img src="{{ site.baseurl}}/img/example-firewall-gce.png" width="350"  alt="">
</p>

### Step 2: Build and install Mesos-DNS

We will build and install Mesos-DNS on node `10.14.245.208`. You can access the node through ssh using:

```
ssh jclouds@10.14.245.208
```

The build process includes installing `go`:

```
sudo apt-get install git-core
wget https://storage.googleapis.com/golang/go1.4.linux-amd64.tar.gz
tar xzf go*
sudo mv go /usr/local/.
export PATH=$PATH:/usr/local/go/bin
export GOROOT=/usr/local/go
export PATH=$PATH:$GOROOT/bin
export GOPATH=$HOME/go
```

Now, we are ready to compile Mesos-DNS:

```
go get github.com/miekg/dns
go get github.com/mesosphere/mesos-dns
cd $GOPATH/src/github.com/mesosphere/mesos-dns
go build -o mesos-dns
sudo mkdir /usr/local/mesos-dns
sudo mv mesos-dns /usr/local/mesos-dns
```

In the same directory (`/usr/local/mesos-dns`), create a file named `config.json` with the following contents: 

```
$ cat /usr/local/mesos-dns/config.json 
{
  "masters": ["10.41.40.151:5050"],
  "refreshSeconds": 60,
  "ttl": 60,
  "domain": "mesos",
  "port": 53,
  "resolvers": ["169.254.169.254","10.0.0.1"],
  "timeout": 5,
  "email": "root.mesos-dns.mesos"
}
```
The `resolvers` field includes the two nameservers listed in the `/etc/resolv.conf` of the nodes in this cluster. 

### Step 3: Launch Mesos-DNS

We will launch Mesos-DNS from the master node
`10.41.40.151`. You can access the node through ssh using:

```
ssh jclouds@10.41.40.151
```

We will use Marathon to launch Mesos-DNS in order to get fault-tolerance. If Mesos-DNS crashes, Marathon will automatically restart it. Create a file `mesos-dns.json` with the following contents:

```
$ more mesos-dns.json 
{
"cmd": "sudo  /usr/local/mesos-dns/mesos-dns -v -config=/usr/local/mesos-dns/config.json",
"cpus": 1.0, 
"mem": 1024,
"id": "mesos-dns",
"instances": 1,
"constraints": [["hostname", "CLUSTER", "10.14.245.208"]]
}

```

Launch Mesos-DNS using:

```
curl -X POST -H "Content-Type: application/json" http://10.41.40.151:8080/v2/apps -d@mesos-dns.json
```

This command instructs Marathon to launch Mesos-DNS on node `10.14.245.208`. The `-v` option allows you to capture detailed warning/error logs for Mesos-DNS that may be useful for debugging. For example, Mesos-DNS will now periodically print in `stdout` information about all the A and SRV records it servers. This can be useful if you are not sure about how tasks and frameworks are named in your setup. You can access the `stdout` and `stderr` for Mesos-DNS through the Mesos webUI, accessible through `http://10.41.40.151:5050` in this example. 

### Step 4: Configure cluster nodes

Next, we will configure all nodes in our cluster to use Mesos-DNS as their DNS server. Access each node through ssh and execute: 


```
sudo sed -i '1s/^/nameserver 10.14.245.208\n /' /etc/resolv.conf
```

We can verify that the configuration is correct and that Mesos-DNS can server DNS queries using the following commands:

```
$ cat /etc/resolv.conf 
nameserver 10.14.245.208
 domain c.myproject.internal.
search c.myprojecct.internal. 267449633760.google.internal. google.internal.
nameserver 169.254.169.254
nameserver 10.0.0.1
$ host www.google.com
www.google.com has address 74.125.70.104
www.google.com has address 74.125.70.147
www.google.com has address 74.125.70.99
www.google.com has address 74.125.70.105
www.google.com has address 74.125.70.106
www.google.com has address 74.125.70.103
www.google.com has IPv6 address 2607:f8b0:4001:c02::93
```

To be 100% sure that Mesos-DNS is actually the server that provided the translation above, we can try:

```
$ sudo apt-get install dnsutils
$ dig www.google.com

; <<>> DiG 9.8.4-rpz2+rl005.12-P1 <<>> www.google.com
;; global options: +cmd
;; Got answer:
;; ->>HEADER<<- opcode: QUERY, status: NOERROR, id: 45045
;; flags: qr rd ra; QUERY: 1, ANSWER: 6, AUTHORITY: 0, ADDITIONAL: 0

;; QUESTION SECTION:
;www.google.com.			IN	A

;; ANSWER SECTION:
www.google.com.		228	IN	A	74.125.201.105
www.google.com.		228	IN	A	74.125.201.103
www.google.com.		228	IN	A	74.125.201.147
www.google.com.		228	IN	A	74.125.201.104
www.google.com.		228	IN	A	74.125.201.106
www.google.com.		228	IN	A	74.125.201.99

;; Query time: 3 msec
;; SERVER: 10.14.245.208#53(10.14.245.208)
;; WHEN: Sat Jan 24 01:03:38 2015
;; MSG SIZE  rcvd: 212
```

The line marked `SERVER` makes it clear that the process we launched to listen to port `53` on node `10.14.245.208` is providing the answer. This is Mesos-DNS. 

### Step 5: Launch nginx using Mesos

Now let's launch a task using Mesos. We will use the nginx webserver using Marathon and Docker. We will use the master node for this:


```
ssh jclouds@10.41.40.151
```

First, create a configuration file for nginx named `nginx.json`:

```
$ cat nginx.json
{
  "id": "nginx",
  "container": {
    "type": "DOCKER",
    "docker": {
      "image": "nginx:1.7.7",
      "network": "HOST"
    }
  },
  "instances": 1,
  "cpus": 1,
  "mem": 640,
  "constraints": [
    [
      "hostname",
      "UNIQUE"
    ]
  ]
}
```

You can launch it on Mesos using: 

```
curl -X POST -H "Content-Type: application/json" http://10.41.40.151:8080/v2/apps -d@nginx.json
```

This will launch nginx on one of the three slaves using docker and host networking. You can use the Marathon webUI to verify it is running without problems. It turns out that Mesos launched it on node `10.114.227.92` and we can verify it works using:

```
$ curl http://10.114.227.92
<!DOCTYPE html>
<html>
<head>
<title>Welcome to nginx!</title>
<style>
    body {
        width: 35em;
        margin: 0 auto;
        font-family: Tahoma, Verdana, Arial, sans-serif;
    }
</style>
</head>
<body>
<h1>Welcome to nginx!</h1>
<p>If you see this page, the nginx web server is successfully installed and
working. Further configuration is required.</p>

<p>For online documentation and support please refer to
<a href="http://nginx.org/">nginx.org</a>.<br/>
Commercial support is available at
<a href="http://nginx.com/">nginx.com</a>.</p>

<p><em>Thank you for using nginx.</em></p>
</body>
</html>
```

### Step 6: Use Mesos-DNS to connect to nginx

Now, let's use Mesos-DNS to communicate with nginx. We will still use the master node:

```
ssh jclouds@10.41.40.151
```

First, let's do a DNS lookup for nginx, using the expected name `nginx.marathon-0.7.6.mesos`. The version number of Marathon is there because it registed with Mesos using name `marathon-0.7.6`. We could have avoided this by launching Marathon using ` --framework_name marathon`:

```
$ dig nginx.marathon-0.7.6.mesos

; <<>> DiG 9.8.4-rpz2+rl005.12-P1 <<>> nginx.marathon-0.7.6.mesos
;; global options: +cmd
;; Got answer:
;; ->>HEADER<<- opcode: QUERY, status: NOERROR, id: 11742
;; flags: qr aa rd ra; QUERY: 1, ANSWER: 1, AUTHORITY: 0, ADDITIONAL: 0

;; QUESTION SECTION:
;nginx.marathon-0.7.6.mesos. IN	A

;; ANSWER SECTION:
nginx.marathon-0.7.6.mesos. 60 IN	A	10.114.227.92

;; Query time: 0 msec
;; SERVER: 10.14.245.208#53(10.14.245.208)
;; WHEN: Sat Jan 24 01:11:46 2015
;; MSG SIZE  rcvd: 96

```

Mesos-DNS informed us that nginx is running on node `10.114.227.92`. Now let's try to connect with it:

```
$ curl http://nginx.marathon-0.7.6.mesos
<!DOCTYPE html>
<html>
<head>
<title>Welcome to nginx!</title>
<style>
    body {
        width: 35em;
        margin: 0 auto;
        font-family: Tahoma, Verdana, Arial, sans-serif;
    }
</style>
</head>
<body>
<h1>Welcome to nginx!</h1>
<p>If you see this page, the nginx web server is successfully installed and
working. Further configuration is required.</p>

<p>For online documentation and support please refer to
<a href="http://nginx.org/">nginx.org</a>.<br/>
Commercial support is available at
<a href="http://nginx.com/">nginx.com</a>.</p>

<p><em>Thank you for using nginx.</em></p>
</body>
</html>
```

We successfully connected with nginx using a logical name. Mesos-DNS works!


### Step 7: Scaling out nginx

Use the Marathon webUI to scale nginx to two instances. Alternatively, relaunch it after editing the json file in step 5 to indicate 2 instances. A minute later, we can look it up again using Mesos-DNS and get:

```
$  dig nginx.marathon-0.7.6.mesos

; <<>> DiG 9.8.4-rpz2+rl005.12-P1 <<>> nginx.marathon-0.7.6.mesos
;; global options: +cmd
;; Got answer:
;; ->>HEADER<<- opcode: QUERY, status: NOERROR, id: 30550
;; flags: qr aa rd ra; QUERY: 1, ANSWER: 2, AUTHORITY: 0, ADDITIONAL: 0

;; QUESTION SECTION:
;nginx.marathon-0.7.6.mesos. IN	A

;; ANSWER SECTION:
nginx.marathon-0.7.6.mesos. 60 IN	A	10.29.107.105
nginx.marathon-0.7.6.mesos. 60 IN	A	10.114.227.92

;; Query time: 1 msec
;; SERVER: 10.14.245.208#53(10.14.245.208)
;; WHEN: Sat Jan 24 01:24:07 2015
;; MSG SIZE  rcvd: 143
```

Now, Mesos-DNS is giving us two A records for the same name, identifying both instances of nginx on  our cluster. 



