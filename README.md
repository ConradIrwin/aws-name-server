A DNS server that serves up your ec2 instances by name.

Usage
=====

```
sudo aws-name-server --domain aws.bugsnag.com --aws-access-key ACCESS --aws-secret-key SECRET
```

This will serve up DNS A records for the following:

* `<name>.aws.bugsnag.com` all your EC2 instances tagged with Name=&lt;name> (usually there's only one)
* `<n>.<name>.aws.bugsnag.com` the nth instances tagged with Name=&lt;name>
* `<role>.role.aws.bugsnag.com` all your EC2 instances tagged with Role=&lt;role> (usually there are many)
* `<n>.<role>.aws.bugsnag.com` the nth instances tagged with Role=&lt;role>

If you run this on an EC2 instance it will serve up private IP addresses to
other machines running inside EC2, and public IP addresses to the global
internet, just like normal Amazon DNS.

Setup
=====

## Installing the binary
*NOTE: you can build this yourself using the normal go workflow if that's easier*

1.  On an EC2 instance Download the latest version from gobuild.

    ```
    wget http://gobuild.io/github.com/ConradIrwin/aws-name-server/master/linux/amd64 -O /tmp/aws-name-server.zip
    unzip aws-name-server.zip
    ```

2. Move the binary into /usr/bin.

    ```
    sudo cp aws-name-server /usr/bin
    sudo chmod +x /usr/bin/aws-name-server
    ```

3. (optional) Set the capabilities of aws-name-server so it doesn't need to run as root.

    ```
    # the cap_net_bind_service capability allows this program to bind to ports below 1024
    # when it us run as a non-root user.
    sudo setcap cap_net_bind_service=+ep "$(which aws-name-server)"
    ```

## AWS Credentials
*NOTE: you can use an existing IAM user with these permissions if that's easier*

1. Log into the AWS web console and navigate to IAM.
2. Create a new user called `aws-name-server`
3. Copy and paste the access key and secret key somewhere safe.
4. Attach a user policy, called `describe-instances-only` with custom text:

    ```
    {
      "Version": "2012-10-17",
      "Statement": [{
        "Action": ["ec2:DescribeInstances"],
        "Effect": "Allow",
        "Resource": "*"
      }]
    }
    ```
5. Make these credentials available to `aws-name-server`, either by:

    * passing command line arguments (this may be insecure on a multi-user system as people can read the keys from ps)
    * booting a new EC2 machine with the IAM role `aws-name-server`
    * exporting the environment variables `$AWS_ACCESS_KEY` and `$AWS_SECRET_KEY` (or legacy `$AWS_ACCESS_KEY_ID` and `$AWS_SECRET_ACCESS_KEY`)
    * creating the [`~/.aws.credentials` file](http://docs.aws.amazon.com/aws-sdk-php/guide/latest/credentials.html#credential-profiles)

## DNS NS Records
*NOTE: if you're not using Route-53, everything will still work if you set up the NS record as described*

Assuming you currently have the DNS domain `example.com`, and you want to lookup AWS name records at `*.aws.example.com`, and you are running
the `aws-name-server` on the EC2 instance with a public CNAME of `ec2-12-34-56-78.compute-1.amazonaws.com`. You need to add
a record to `example.com`'s DNS server that looks like this: `aws.example.com    300   IN   NS`.

### Using Route 53
1. Log into the AWS web console and navigate to Route 53.
2. Select your Record Set and then click "Go to Record Sets"
3. Click "Create Record Set"
4. Set the name to "aws" or similar
5. Set the type to "NS"
6. Set the Value to the Public CNAME of the server running `aws-name-server` (e.g. ec2-12-34-56-78.compute-1.amazonaws.com)
7. Click "Save Record Set"
