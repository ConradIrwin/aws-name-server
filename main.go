package main

import (
	"flag"
	"github.com/miekg/dns"
	"github.com/mitchellh/goamz/aws"
	"github.com/mitchellh/goamz/ec2"
	"log"
	"net"
	"strings"
)

var domain = flag.String("domain", "internal.example.com", "the domain to serve")
var ttl = flag.Int("ttl", 300, "the domain to serve")

type EC2Cache struct{
	*ec2.EC2
}

func (cache *EC2Cache) Lookup(name string) (ret []net.IP) {
	filter := ec2.NewFilter()
	filter.Add("tag:Name", name)

	result, err := cache.Instances(nil, filter)

	if err != nil {
		log.Println("Error: " + err.Error())
		return
	}

	for _, reservation := range result.Reservations {
		for _, instance := range reservation.Instances {
			ret = append(ret, net.ParseIP(instance.PublicIpAddress))
		}
	}

	return
}

func main() {

	auth, err := aws.EnvAuth()
	if err != nil {
		panic(err)
	}

	cache := &EC2Cache{ec2.New(auth, aws.USEast)}

	log.Printf("responding to *.%s from port :53", *domain)

	server := &dns.Server{Addr: ":53", Net: "udp"}

	dns.HandleFunc(*domain, func (w dns.ResponseWriter, request *dns.Msg) {

		response := new(dns.Msg)
		response.SetReply(request)

		for _, msg := range request.Question {
			log.Printf("%v %#v %v (id=%v)", dns.TypeToString[msg.Qtype], msg.Name, w.RemoteAddr(), request.Id)
			name := strings.Split(msg.Name, ".")[0]
			for _, ip := range cache.Lookup(name) {
				response.Answer = append(response.Answer, &dns.A{
					Hdr: dns.RR_Header{Name: msg.Name, Rrtype: dns.TypeA, Class: dns.ClassINET, Ttl: uint32(*ttl)},
					A: ip,
				})
			}
		}

		w.WriteMsg(response)
	})

	if err := server.ListenAndServe(); err != nil {
		panic(err)
	}
}
