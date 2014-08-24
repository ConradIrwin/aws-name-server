package main

import (
	"flag"
	"fmt"
	"log"
	"os"
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
	hostname := flag.String("hostname", "", "the public hostname of this server (e.g. ec2-12-34-56-78.compute-1.amazonaws.com")
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

	server := NewEC2Server(*domain, *hostname, cache)

	log.Printf("Serving DNS for *.%s from port :53", server.domain)

	go server.listenAndServe(":53", "udp")
	server.listenAndServe(":53", "tcp")
}
