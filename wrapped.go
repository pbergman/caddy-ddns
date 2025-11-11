package dyndns_handler

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/caddyserver/caddy/v2"
	"github.com/caddyserver/caddy/v2/caddyconfig"
	"github.com/caddyserver/caddy/v2/caddyconfig/caddyfile"
	"github.com/libdns/libdns"
)

type StaticZonesProvider struct {
	ZonesRaw    []string        `json:"zones,omitempty"`
	ProviderRaw json.RawMessage `json:"provider,omitempty" caddy:"namespace=dns.providers inline_key=name"`
	provider    BaseProvider
	zones       []libdns.Zone
}

func (s *StaticZonesProvider) SetRecords(ctx context.Context, zone string, recs []libdns.Record) ([]libdns.Record, error) {
	return s.provider.SetRecords(ctx, zone, recs)
}

func (s *StaticZonesProvider) AppendRecords(ctx context.Context, zone string, recs []libdns.Record) ([]libdns.Record, error) {
	return s.provider.AppendRecords(ctx, zone, recs)
}

func (s *StaticZonesProvider) GetRecords(ctx context.Context, zone string) ([]libdns.Record, error) {
	return s.provider.GetRecords(ctx, zone)
}

func (s *StaticZonesProvider) DeleteRecords(ctx context.Context, zone string, recs []libdns.Record) ([]libdns.Record, error) {
	return s.provider.DeleteRecords(ctx, zone, recs)
}

func (s *StaticZonesProvider) ListZones(ctx context.Context) ([]libdns.Zone, error) {
	return s.zones, nil
}

func init() {
	caddy.RegisterModule(StaticZonesProvider{})
}

func (StaticZonesProvider) CaddyModule() caddy.ModuleInfo {
	return caddy.ModuleInfo{
		ID: "dns.providers.ddns.static_zones",
		New: func() caddy.Module {
			return new(StaticZonesProvider)
		},
	}
}

func (p *StaticZonesProvider) Provision(ctx caddy.Context) error {

	if len(p.ProviderRaw) == 0 {
		return fmt.Errorf("no DNS provider defined")
	}

	val, err := ctx.LoadModule(p, "ProviderRaw")

	if err != nil {
		return fmt.Errorf("failed loading DNS provider module: %v", err)
	}

	if _, ok := val.(BaseProvider); !ok {
		return fmt.Errorf("expected provider module to implement libdns.{RecordSetter, RecordDeleter, RecordAppender, RecordGetter}")
	}

	p.provider = val.(BaseProvider)
	p.zones = make([]libdns.Zone, len(p.ZonesRaw))

	for i, z := range p.ZonesRaw {
		p.zones[i] = libdns.Zone{Name: z}
	}

	p.ZonesRaw = nil

	return nil
}

// UnmarshalCaddyfile sets up the Wrapped DNS provider from Caddyfile tokens. Syntax:
//
//	ddns.static_zones {
//	    provider 	<name> ...
//		zones		<zone ...>
//	}
func (p *StaticZonesProvider) UnmarshalCaddyfile(d *caddyfile.Dispenser) error {

	if !d.Next() {
		return d.ArgErr()
	}

	for d.NextBlock(0) {
		switch d.Val() {
		case "provider":

			if !d.NextArg() {
				return d.ArgErr()
			}

			var name = d.Val()

			encoder, err := caddyfile.UnmarshalModule(d, "dns.providers."+name)

			if err != nil {
				return err
			}

			p.ProviderRaw = caddyconfig.JSONModuleObject(encoder, "name", name, nil)
		case "zones":
			var args = d.RemainingArgs()

			if len(args) == 0 {
				return d.Errf("must specify at least one zone")
			}

			p.ZonesRaw = args
		}
	}

	return nil
}

// Interface guards
var (
	_ caddyfile.Unmarshaler = (*StaticZonesProvider)(nil)
	_ caddy.Provisioner     = (*StaticZonesProvider)(nil)
	_ Provider              = (*StaticZonesProvider)(nil)
)
