# DDNS Handler for caddy 


This module implements a handler capable of processing [Dyn-style DDNS update requests](https://help.dyn.com/perform-update.html) (which a lot of routers have built in). 

It uses DNS provider modules from the [`caddy-dns`](https://github.com/caddy-dns) to update the requested DNS records.

To determine which DNS records can be modified, the underlying `libdns` provider must implement the [`libdns.ZoneLister`](https://github.com/libdns/libdns/blob/master/libdns.go#L250) interface.

For providers that do **not** implement `ZoneLister`, a [wrapper](./wrapped.go) is included to provide compatible zone-listing functionality.




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