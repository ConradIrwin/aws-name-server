package main

import (
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"time"

	"github.com/mitchellh/goamz/aws"
)

const USAGE = `Usage: aws-name-server --domain <domain>
                     [ --hostname <hostname>
                       --aws-region us-east-1
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
	domain := flag.String("domain", "", "the domain heirarchy to serve (e.g. aws.example.com)")
	hostname := flag.String("hostname", "", "the public hostname of this server (e.g. ec2-12-34-56-78.compute-1.amazonaws.com)")
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

	hostnameFuture := getHostname()

	cache, err := NewEC2Cache(*region, *accessKey, *secretKey)
	if err != nil {
		log.Fatalf("FATAL: %s", err)
	}

	if *hostname == "" {
		*hostname = <-hostnameFuture
	}

	server := NewEC2Server(*domain, *hostname, cache)

	log.Printf("Serving DNS for *.%s from %s port 53", server.domain, server.hostname)

	go checkNSRecordMatches(server.domain, server.hostname)

	go server.listenAndServe(":53", "udp")
	server.listenAndServe(":53", "tcp")
}

func getHostname() chan string {
	result := make(chan string)
	go func () {

		// This can be slow on non-EC2-instances
		if hostname, err := aws.GetMetaData("public-hostname"); err == nil {
			result <- string(hostname)
			return
		}

		if hostname, err := os.Hostname(); err == nil {
			result <- hostname
			return
		}

		result <- "localhost"
	}()
	return result
}

// checkNSRecordMatches does a spot check for DNS misconfiguration, and prints a warning
// if using it for DNS is likely to be broken.
func checkNSRecordMatches(domain, hostname string) {

	time.Sleep(1 * time.Second)

	results, err := net.LookupNS(domain)

	if err != nil {
		log.Printf("|WARN| No working NS records found for %s", domain)
		log.Printf("|WARN| You can still test things using `dig example.%s @%s`, but you won't be able to resolve hosts directly.", domain, hostname)
		log.Printf("|WARN| See https://github.com/ConradIrwin/aws-name-server for instructions on setting up NS records.")
		return
	}

	matched := false

	for _, record := range results {
		if record.Host == hostname {
			matched = true
		}
	}

	if !matched {
		log.Printf("|WARN| The NS record for %s points to: %s", domain, results[0].Host)
		log.Printf("|WARN| But --hostname is: %s", hostname)
		log.Printf("|WARN| These hostnames must match if you want DNS to work properly.")
		log.Printf("|WARN| See https://github.com/ConradIrwin/aws-name-server for instructions on NS records.")
	}
}
