package dyndns_handler

import (
	"context"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"strings"
	"sync"
	"time"
)

type IPPrefixList []netip.Prefix

func (t *IPPrefixList) Contains(ip netip.Addr) bool {
	for _, prefix := range *t {
		if prefix.Contains(ip) {
			return true
		}
	}
	return false
}

// getIp first checks if an IP address is provided in the request query.
// If it’s omitted or cannot be parsed, it attempts to determine the
// client’s IP address by processing the X-Forwarded-For header.
// If no valid IP is found (or the header is missing), it will either
// return the remote address from the connection or attempt to retrieve
// the WAN address depending on config (NoLocalIp).
func getIp(query url.Values, remoteAddr string, header http.Header, config *Handler) (netip.Addr, error) {

	// https://help.dyn.com/perform-update.html
	if query.Has("myip") {
		if value, err := netip.ParseAddr(query.Get("myip")); err == nil {
			return value, nil
		}
	}

	remote, err := netip.ParseAddrPort(remoteAddr)

	if err != nil {
		return netip.Addr{}, err
	}

	var locals = sync.OnceValue(func() *IPPrefixList {
		var locals IPPrefixList = make([]netip.Prefix, 5)

		// see https://datatracker.ietf.org/doc/html/rfc1918

		for idx, x := range []string{"10.0.0.0/8", "172.16.0.0/12", "192.168.0.0/16", "127.0.0.1/8", "fd00::/8"} {
			if prefix, _ := netip.ParsePrefix(x); prefix.IsValid() {
				locals[idx] = prefix
			}
		}

		return &locals
	})

	if nil == config || nil == config.TrustedRemotes || false == config.TrustedRemotes.Contains(remote.Addr()) {

		if nil != config && config.NoLocalIp && locals().Contains(remote.Addr()) {
			return getRemoteIp()
		}

		return remote.Addr(), nil
	}

	var list = getIpAddrFromList(header.Values("x-forwarded-for"))

	for i := len(list) - 1; i >= 0; i-- {
		if list[i].IsValid() && false == config.TrustedRemotes.Contains(list[i]) && (config.NoLocalIp && false == locals().Contains(list[i])) {
			return list[i], nil
		}
	}

	if config.NoLocalIp {
		return getRemoteIp()
	}

	return remote.Addr(), nil
}

// getRemoteIp will do dns query to get remote ip similar
// to the command
//
//	dig +short @resolver1.opendns.com myip.opendns.com
func getRemoteIp() (netip.Addr, error) {

	var resolver = &net.Resolver{
		PreferGo: true,
		Dial: func(ctx context.Context, network, address string) (net.Conn, error) {

			var dialer = net.Dialer{
				Timeout: time.Millisecond * time.Duration(10000),
			}

			return dialer.DialContext(ctx, network, "resolver1.opendns.com:53")
		},
	}

	ip, err := resolver.LookupHost(context.Background(), "myip.opendns.com")

	if err != nil {
		return netip.Addr{}, err
	}

	if len(ip) == 0 {
		return netip.Addr{}, nil
	}

	return netip.ParseAddr(ip[0])
}

func getIpAddrFromList(list []string) []netip.Addr {

	if 0 == len(list) {
		return nil
	}

	var items = make([]netip.Addr, 0)

	for i, c := 0, len(list); i < c; i++ {

		var values = strings.Split(list[i], ",")

		for x, y := 0, len(values); x < y; x++ {
			// ignore error for now and check later with IsValid
			item, _ := netip.ParseAddr(strings.TrimSpace(values[x]))
			items = append(items, item)
		}
	}

	return items
}
