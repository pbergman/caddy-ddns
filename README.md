# DDNS Handler for caddy 

This module implements a handler capable of processing [Dyn-style DDNS update requests](https://help.dyn.com/perform-update.html), which many routers support natively and can be used to monitor LAN or WAN IP addresses and update DNS records accordingly.

It uses the DNS provider modules from [`caddy-dns`](https://github.com/caddy-dns) to update the requested DNS records. To determine which records can be modified, the underlying `libdns` provider must implement the [`libdns.ZoneLister`](https://github.com/libdns/libdns/blob/master/libdns.go#L250) interface.

For providers that do **not** implement `ZoneLister`, a [wrapper](./wrapped.go) is included to provide compatible zone-listing functionality, ensuring consistent behavior across all supported providers.

## IP Resolvin

The handler will first check if the query parameter `myip` exists. If it is missing or cannot be parsed as a valid IP address, it will fall back to the client’s `address`.

When running behind a proxy, the client’s remote address may belong to the proxy and therefore be invalid. To handle this, there are two options: `trusted_remotes` and `no_local_ip`.

* `trusted_remotes` allows you to define a list of trusted IP ranges whose `X-Forwarded-For` headers will be validated.
* `no_local_ip` ensures that only a public IP is returned. If no valid IP can be resolved, it will return the machine’s public IP.

For example, if you are running behind a local proxy:

```caddyfile
ddns /nuc/update {
    trusted_remotes 127.0.0.1/8
    ...
}
```

This configuration trusts the `X-Forwarded-For` header for requests from `127.0.0.1`. In some cases, this may resolve to a local IP, which can be acceptable. In scenarios where only a public IP should be used, enabling `no_local_ip` guarantees that the returned IP is public.

## Authorisation

Because clients expect a `badauth` response **with a 200 HTTP status** when authentication fails, we cannot rely on the standard `basic_auth` directive. Instead, the handler uses a simple username-password map to authenticate incoming requests.

For example:

```caddyfile
example.com {

    tls {
        dns mijnhost <APIKEY>
    }

    ddns /nuc/update {
        users {
            foo bar
        }
        providers {
            mijnhost <APIKEY>
            ...
        }
    }
}
```

With this configuration:

```bash
~/ curl 'https://example.com/nuc/update?hostname=foo.example.com&myip=127.0.0.1'
badauth
```

When providing valid credentials:

```bash
~/ curl 'https://foo:bar@example.com/nuc/update?hostname=foo.example.com&myip=127.0.0.1'
nochg 127.0.0.1
```

This approach ensures compatibility with Dyn-style DDNS clients while allowing per-user authentication.

## DNS Providers

To also support providers that do **not** implement the `libdns.ZoneLister` interface, a DNS wrapper provider is included. This wrapper can wrap around any `caddy-dns` provider and return a predefined list of zones when the supported zones are queried.

For example:

```caddyfile
ddns.static_zones {
    provider <provider_name>
    zones    bar.com foo.com
}
```

With this configuration, the wrapped provider will be called whenever a request comes in for hostnames in the `bar.com` or `foo.com` zones.

## Build with xcaddy
```
$ xcaddy build --with github.com/pbergman/caddy-ddns
```

##  Config (Caddyfile)
```
example.com {

    tls {
        dns mijnhost <APIKEY>
    }

    ddns /nuc/update {
        users {
            foo bar
        }
        providers {
            mijnhost <APIKEY>
            ....
        }
    }
}
```

or with static zones 

```
example.com {

    tls {
        dns mijnhost <APIKEY>
    }

    ddns /nuc/update {
        providers {
            ddns.static_zones {
                provider mijnhost APIKEY
                zones    example.nl foo.com
            }
        }
    }
}

```

### Example Configuration (Vyatta / EdgeOS)

For EdgeRouters or any operating system based on **Vyatta OS**, the following commands configure the DDNS client to update the DNS records for the domains `bar.example.com` and `foo.example.com` using the ip on network interface **`eth1`**.

```bash
configure
set service dns dynamic interface eth1 service custom-caddy_ddns host-name bar.example.com
set service dns dynamic interface eth1 service custom-caddy_ddns host-name foo.example.com
set service dns dynamic interface eth1 service custom-caddy_ddns protocol dyndns2
set service dns dynamic interface eth1 service custom-caddy_ddns login foo
set service dns dynamic interface eth1 service custom-caddy_ddns password bar
set service dns dynamic interface eth1 service custom-caddy_ddns server example.com
commit; save; exit
```

#### Force an update

To manually trigger an update of the DDNS records for `eth1`, run:

```bash
update dns dynamic interface eth1
```

#### Show DDNS Status

To check the current DDNS status and update timestamps, run:

```bash
show dns dynamic status
```