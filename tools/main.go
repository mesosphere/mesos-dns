package main

import (
	"fmt"
	"github.com/miekg/dns"
	"log"
	"time"
)

func query(dom string) {

	nameserver := "127.0.0.1:8053"

	qt := dns.TypeA
	qc := uint16(dns.ClassINET)

	c := new(dns.Client)
	c.Net = "udp"

	m := new(dns.Msg)
	m.Question = make([]dns.Question, 1)
	m.Question[0] = dns.Question{dns.Fqdn(dom), qt, qc}

	_, _, err := c.Exchange(m, nameserver)
	// fmt.Println(rtt)
	if err != nil {
		fmt.Println(err)
	}

	// fmt.Println(in)
}

func main() {

	start := time.Now()

	cnt := 10000

	for i := 0; i < cnt; i++ {
		go query("bob.mesos")
	}

	elapsed := time.Since(start)
	log.Printf("benching took %s", elapsed)
	log.Printf("doing %s/%s rps", cnt, elapsed)
}
