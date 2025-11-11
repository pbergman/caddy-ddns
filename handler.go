package dyndns_handler

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/netip"
	"reflect"
	"strings"

	"github.com/caddyserver/caddy/v2"
	"github.com/caddyserver/caddy/v2/caddyconfig"
	"github.com/caddyserver/caddy/v2/caddyconfig/caddyfile"
	"github.com/caddyserver/caddy/v2/caddyconfig/httpcaddyfile"
	"github.com/caddyserver/caddy/v2/modules/caddyhttp"
	"github.com/libdns/libdns"
	"go.uber.org/zap"
)

type Handler struct {

	// The provider configurations with which will be used
	// to update records incoming reqeust.
	ProvidersRaw []json.RawMessage `json:"providers,omitempty" caddy:"namespace=dns.providers inline_key=name"`

	// List of trusted remotes which will be used to determine
	// the client ip based on x-forwarded-for header
	TrustedRemotes *IPPrefixList `json:"trusted_remotes"`

	// When true, we only satisfy by a public ip when trying
	// to determine the client ip. This means when no ip is
	// found, it will try to resolve "this" remote ip
	NoLocalIp bool `json:"no_local_ip"`

	// Because https://help.dyn.com/return-codes.html specifies
	// that we return "badauth" (with status 200) we cannot use
	// the basic_auth directive as it will return 401 as expected
	// in a normal authentication request.
	//
	// So a username – password map will be used for authentication
	// and when left empty, all requests will be allowed.
	//
	// At this moment the passwords are stored in plain text. Since
	// the dns provider API tokens and keys are also plain text,
	// there’s currently no real benefit for adding additional
	// hashing or encryption mechanisms.
	Users map[string]string `json:"users"`

	providers []Provider
	logger    *zap.Logger
}

func init() {
	httpcaddyfile.RegisterHandlerDirective("ddns", parseCaddyfile)
	httpcaddyfile.RegisterDirectiveOrder("ddns", "before", "basic_auth")
	caddy.RegisterModule(Handler{})
}

func (Handler) CaddyModule() caddy.ModuleInfo {
	return caddy.ModuleInfo{
		ID:  "http.handlers.ddns",
		New: func() caddy.Module { return new(Handler) },
	}
}

func (h *Handler) Provision(ctx caddy.Context) error {

	if nil == h.providers {
		h.providers = make([]Provider, 0)
	}

	if len(h.ProvidersRaw) == 0 {
		return fmt.Errorf("no DNS providers defined")
	}

	val, err := ctx.LoadModule(h, "ProvidersRaw")

	if err != nil {
		return fmt.Errorf("loading DNS providers module: %v", err)
	}

	var interfaces = []reflect.Type{
		reflect.TypeOf((*libdns.RecordAppender)(nil)).Elem(),
		reflect.TypeOf((*libdns.RecordGetter)(nil)).Elem(),
		reflect.TypeOf((*libdns.RecordSetter)(nil)).Elem(),
		reflect.TypeOf((*libdns.RecordDeleter)(nil)).Elem(),
		reflect.TypeOf((*libdns.ZoneLister)(nil)).Elem(),
	}

	for i, c := 0, len(val.([]interface{})); i < c; i++ {

		var value = reflect.ValueOf(val.([]interface{})[i])
		var missing = make([]string, 0)
		var zoneHint = false

		if value.CanAddr() {
			value = value.Elem()
		}

		for _, expecting := range interfaces {

			if false == value.Type().Implements(expecting) {
				missing = append(missing, expecting.Name())

				if "ZoneLister" == expecting.Name() {
					zoneHint = true
				}
			}
		}

		if len(missing) > 0 {

			var err = fmt.Sprintf("DNS provider %s should implement ", ProviderName(value.Interface().(caddy.Module)))

			if len(missing) == 1 {
				err += "libdns." + missing[0]
			} else {
				err += "libdns.{" + strings.Join(missing, ", ") + "}"
			}

			if zoneHint {
				err += " (use provider ddns.static_zones to manually define zones)"
			}

			return errors.New(err)

		}

		h.providers = append(h.providers, value.Interface().(Provider))
	}

	h.logger = ctx.Logger()

	return nil
}

func parseCaddyfile(h httpcaddyfile.Helper) (caddyhttp.MiddlewareHandler, error) {
	var handler = new(Handler)

	if err := handler.UnmarshalCaddyfile(h.Dispenser); err != nil {
		return nil, err
	}

	return handler, nil
}

// UnmarshalCaddyfile sets up the handler from Caddyfile tokens. Syntax:
//
//	ddns {
//	    provider 	{
//	    	<name> ...
//		}
//		no_local_ip
//		users {
//			username password
//		}
//		trusted_remotes <ip prefix>...
//	}
func (h *Handler) UnmarshalCaddyfile(d *caddyfile.Dispenser) error {

	if !d.Next() {
		return d.ArgErr()
	}

	for d.NextBlock(0) {
		switch d.Val() {
		case "providers":

			h.ProvidersRaw = make([]json.RawMessage, 0)

			for nesting := d.Nesting(); d.NextBlock(nesting); {

				var name = d.Val()

				encoder, err := caddyfile.UnmarshalModule(d, "dns.providers."+name)

				if err != nil {
					return err
				}

				h.ProvidersRaw = append(h.ProvidersRaw, caddyconfig.JSONModuleObject(encoder, "name", name, nil))
			}
		case "no_local_ip":
			h.NoLocalIp = true
		case "trusted_remotes":
			var args = d.RemainingArgs()
			if len(args) == 0 {
				return d.Errf("must specify at least one trusted remote")
			}
			h.TrustedRemotes = new(IPPrefixList)
			for i, c := 0, len(args); i < c; i++ {
				prefix, err := netip.ParsePrefix(args[i])

				if err != nil {
					return err
				}

				*h.TrustedRemotes = append(*h.TrustedRemotes, prefix)
			}
		case "users":
			h.Users = make(map[string]string)
			for nesting := d.Nesting(); d.NextBlock(nesting); {
				var name = d.Val()
				var passwd string
				d.Args(&passwd)
				if d.NextArg() {
					return d.ArgErr()
				}
				if _, x := h.Users[name]; x {
					return d.Errf("duplicate user %s", name)
				}
				h.Users[name] = passwd
			}
		}
	}

	return nil
}

func ProviderName(module caddy.Module) string {
	var name = module.CaddyModule().ID.Name()

	if v, ok := module.(*StaticZonesProvider); ok {
		name += "(" + v.provider.(caddy.Module).CaddyModule().ID.Name() + ")"
	}

	return module.CaddyModule().ID.Namespace() + "." + name
}

var (
	_ caddy.Provisioner           = (*Handler)(nil)
	_ caddyfile.Unmarshaler       = (*Handler)(nil)
	_ caddyhttp.MiddlewareHandler = (*Handler)(nil)
)
