package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/miekg/dns"
)

const USAGE = `Usage: aws-name-server --domain <domain>
                     [ --aws-region us-east-1
					   --aws-access-key <access-key>
                       --aws-secret-key <secret-key> ]

aws-name-server --domain internal.example.com will serve DNS requests for:

 <name>.internal.example.com          — all ec2 instances tagged with Name=<name>
 <role>.role.internal.example.com     — all ec2 instances tagged with Role=<role>
 <n>.<name>.internal.example.com      — <n>th instance tagged with Name=<name>
 <n>.<role>.role.internal.example.com — <n>th instance tagged with Role=<role>

For more details see https://github.com/ConradIrwin/aws-name-server`

const CAPABILITIES = `FATAL

You need to give this program permission to bind to port 53.

Using capabilities (recommended):
 $ sudo setcap cap_net_bind_service=+ep "$(which aws-name-server)"

Just run it as root (not recommended):
 $ sudo aws-name-server

`

func main() {
	domain := flag.String("domain", "", "the domain heirarchy to serve (e.g. internal.bugsnag.com)")
	help := flag.Bool("help", false, "show help")

	region := flag.String("aws-region", "us-east-1", "The AWS Region")
	accessKey := flag.String("aws-access-key-id", "", "The AWS Access Key Id")
	secretKey := flag.String("aws-secret-access-key", "", "The AWS Secret Key")

	flag.Parse()

	if *domain == "" {
		fmt.Println(USAGE)
		log.Fatalf("missing required parameter: --domain")
	} else if *help {
		fmt.Println(USAGE)
		os.Exit(0)
	}

	cache, err := NewEC2Cache(*region, *accessKey, *secretKey)
	if err != nil {
		log.Fatalf("FATAL: %s", err)
	}

	suffix := "." + *domain + "."

	log.Printf("Serving DNS for *.%s from port :53", *domain)

	dns.HandleFunc(*domain, func(w dns.ResponseWriter, request *dns.Msg) {
		handleDNSRequest(w, request, cache, suffix)
	})

	go bootServer(":53", "udp")
	bootServer(":53", "tcp")
}

func bootServer(port string, net string) {
	server := &dns.Server{Addr: port, Net: net}
	if err := server.ListenAndServe(); err != nil {
		if strings.Contains(err.Error(), "permission denied") {
			log.Printf(CAPABILITIES)
		}
		log.Fatalf("%s", err)
	}
}

func handleDNSRequest(w dns.ResponseWriter, request *dns.Msg, cache *EC2Cache, suffix string) {
	response := new(dns.Msg)
	response.SetReply(request)

	for _, msg := range request.Question {

		log.Printf("%v %#v %v (id=%v)", dns.TypeToString[msg.Qtype], msg.Name, w.RemoteAddr(), request.Id)

		if !strings.HasSuffix(msg.Name, suffix) {
			log.Printf("ERROR: missing suffix: %s", msg.Name)
			continue
		}

		if msg.Qtype != dns.TypeA || msg.Qtype != dns.TypeCNAME {
			continue
		}

		parts := strings.Split(strings.TrimSuffix(msg.Name, suffix), ".")

		nth := 0
		tag := LOOKUP_NAME

		// handle role lookup, e.g. web.role.internal
		if len(parts) > 1 {
			if parts[len(parts)-1] == "role" {
				tag = LOOKUP_ROLE
				parts = parts[:len(parts)-1]
			}
		}

		// handle nth lookup, e.g. 1.web.internal
		if len(parts) > 1 {
			if i, err := strconv.Atoi(parts[0]); err == nil && i > 0 {
				nth = i
				parts = parts[1:]
			}
		}

		if len(parts) != 1 || parts[0] == "" {
			log.Printf("ERROR: badly formed: %s %#v", msg.Name, parts)
			continue
		}

		results := cache.Lookup(tag, parts[0])

		if nth != 0 {
			if nth > len(results) {
				results = results[0:0]
			} else {
				results = results[nth-1 : nth]
			}
		}

		for _, record := range results {
			ttl := uint32(record.TTL(time.Now()) / time.Second)

			if record.CName != "" {
				response.Answer = append(response.Answer, &dns.CNAME{
					Hdr:    dns.RR_Header{Name: msg.Name, Rrtype: dns.TypeCNAME, Class: dns.ClassINET, Ttl: ttl},
					Target: record.CName,
				})

			} else if record.PublicIP != nil {
				response.Answer = append(response.Answer, &dns.A{
					Hdr: dns.RR_Header{Name: msg.Name, Rrtype: dns.TypeA, Class: dns.ClassINET, Ttl: ttl},
					A:   record.PublicIP,
				})
			} else {
				response.Answer = append(response.Answer, &dns.A{
					Hdr: dns.RR_Header{Name: msg.Name, Rrtype: dns.TypeA, Class: dns.ClassINET, Ttl: ttl},
					A:   record.PrivateIP,
				})
			}
		}
	}

	w.WriteMsg(response)

}
