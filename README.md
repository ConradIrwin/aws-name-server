A DNS server that serves up your ec2 instances by name.

Installation
============

```shell
go get github.com/ConradIrwin/aws-name-server
```

TODO: downloads for 64-bit linux.

Usage
=====

DNS servers need to be run on port 53, so you will almost certainly need to run this as root.

It requires two environment variables:

```
# An AWS key pair. It should have permission to run describe instances.
export AWS_ACCESS_KEY=...
export AWS_SECRET_KEY=...

# Run the server on port 53, serving hosts *.internal.example.com
aws-name-server --domain internal.example.com
```

Now you can look up `web.internal.example.com`, and aws-name-server will look up your AWS dashboard
and return the public IP address of the host.

TODO: return private IP for requests within ec2.

Finally you need to configure `NS` records for `internal.example.com` that
point to this name server. (And probably allow port 53 through your firewall).

TODO: route53 instructions.
