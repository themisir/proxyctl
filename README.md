# Proxy Control CLI

Command line utility to easily access to kubernetes services using human readable domain names from your device.

```shell script
proxyctl [--listen=127.0.0.1:1234] [--no-hosts] [--insecure] <manifest file>
```

**`--listen`** - Listening address. You can use ":80" value to access services by directly typing url-s (without port number), but in some cases port number 80 might used by different apps, so you have to use different port numbers.

**`--no-hosts`** - **proxyctl** will add new records to hosts file which is used as local DNS. Please note that updating hosts file requires super user permissions. 

**`--insecure`** - Disables SSL verification.

## Manifest file

Manifest files used to store settings and service routing rules.

You can specify manifest file as command line argument or by placing a file named `proxy.yaml` in `$(pwd)` directory or `.proxy.yaml` in user home directory.

```yaml
# Defines which inet address to listen. ":80" means any host on port 80.
# Please note that port numbers less than 1024 requires administrator 
# (sudo) permission. 
listen: ":80"

# Disable SSL validation. Not recommended but if you are using self signed
# certificates that doesn't known by your machine you can enable this option.
insecure: true

# Target services.
services:
  # - name: myservice.com  # Domain name to access the service locally.
  #                        # ".local" suffix will be appended if given
  #                        # value does not contains any gTLD.
  #   target: svc/myapp    # Service or pod name
  #   namespace: default   # Kubernetes namespace that contains given target
  #   port: 5000           # Target port
  #   protocol: http       # Service protocol (http | https)
  - name: rabbitmq
    target: svc/rabbitmq
    namespace: mq
    port: 15672
  - name: argocd
    target: svc/argocd-server
    namespace: argocd
    port: 80
    protocol: https
```