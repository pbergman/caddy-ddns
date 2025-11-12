package dyndns_handler

import (
	"errors"
	"fmt"
	"net/http"
	"net/netip"
	"net/url"
	"strings"
	"time"

	"github.com/caddyserver/caddy/v2"
	"github.com/caddyserver/caddy/v2/modules/caddyhttp"
	"github.com/libdns/libdns"
	"go.uber.org/zap"
)

// ServeHTTP will handle incoming request and return 200 with code as described in
//
//	https://help.dyn.com/return-codes.html
func (h *Handler) ServeHTTP(response http.ResponseWriter, request *http.Request, next caddyhttp.Handler) error {

	h.logger.Debug(
		fmt.Sprintf("%s %s", request.Method, request.RequestURI),
	)

	if false == h.authorize(request) {
		return h.writeReturnCode(response, nil, nil, BadAuthentication)
	}

	var query = request.URL.Query()

	if false == query.Has("hostname") {
		// If no hostnames were specified, **notfqdn** will be returned once.
		return h.writeReturnCode(response, nil, nil, NotFullyQualifiedDomainName)
	}

	var ip netip.Addr
	var err error
	var hosts, results = getHosts(query)
	var lock = NewSemaphore(5)

	if ip, err = getIp(query, request.RemoteAddr, request.Header, h); err != nil {
		if x := h.writeReturnCode(response, nil, hosts, h.setReturnCodes(results, DNSError)...); x != nil {
			return errors.Join(err, x)
		}
		return err
	}

	h.logger.Info(
		"ddns update request",
		zap.String("ip", ip.String()),
		zap.Strings("hosts", hosts),
		zap.String("user agent", request.Header.Get("user-agent")),
	)

	var zones = getAvailableZones(request.Context(), h.providers, lock, h.logger)

	type job struct {
		provider BaseProvider
		items    map[string][]libdns.Record
		result   map[string][]libdns.Record
		errors   map[string]error
	}

	var queue = make([]*job, 0)

	for idx, items := range h.makeChangeLists(hosts, ip, zones, &results) {

		lock.Lock()

		var work = &job{
			provider: h.providers[idx],
			items:    items,
			result:   make(map[string][]libdns.Record),
			errors:   make(map[string]error),
		}

		queue = append(queue, work)

		go func(job *job) {
			defer lock.Unlock()
			for zone, records := range items {
				job.result[zone], job.errors[zone] = job.provider.SetRecords(request.Context(), zone, records)
			}

		}(work)
	}

	lock.Wait()

	for i, c := 0, len(queue); i < c; i++ {
		for zone, records := range queue[i].items {
			if queue[i].errors[zone] != nil {
				h.logger.Error("setting records failed", zap.String("zone", zone), zap.Error(queue[i].errors[zone]))
				h.setReturnCodesForItems(&results, records, DNSError, zone, hosts)
			} else if len(queue[i].result[zone]) > 0 {
				h.setReturnCodesForItems(&results, records, Good, zone, hosts)
			}
		}
	}

	return h.writeReturnCode(response, &ip, hosts, results...)
}

func (h *Handler) authorize(request *http.Request) bool {

	if len(h.Users) == 0 {
		h.logger.Debug("authorisation ok, no user configured")
		return true
	}

	user, passwd, ok := request.BasicAuth()

	if !ok {
		h.logger.Debug("authorisation failed, no or invalid authorization header")
		return false
	}

	if v, ok := h.Users[user]; !ok || v != passwd {
		h.logger.Debug("authorisation failed, user not exists or password invalid")
		return false
	}

	h.logger.Debug("authorisation ok")
	return true
}

func getHosts(query url.Values) ([]string, []ReturnCode) {
	var hosts = strings.Split(query.Get("hostname"), ",")
	var results = make([]ReturnCode, len(hosts))

	for idx, _ := range hosts {
		results[idx] = NoChange
	}

	return hosts, results
}

func getHostIdx(hosts []string, name string, zone string) int {

	var hostname = strings.TrimSuffix(libdns.AbsoluteName(name, zone), ".")

	for i, c := 0, len(hosts); i < c; i++ {
		if hostname == hosts[i] {
			return i
		}
	}

	return -1
}

func (h *Handler) makeChangeLists(hosts []string, ip netip.Addr, zones [][]string, result *[]ReturnCode) map[int]map[string][]libdns.Record {

	var updates = make(map[int]map[string][]libdns.Record)

hostnames:
	for idx, hostname := range hosts {

		for x, plugin := range h.providers {

			for _, zone := range zones[x] {
				if strings.HasSuffix(hostname, "."+zone) {

					h.logger.Debug(fmt.Sprintf("hostname %s matches zone %s (module %s)", hostname, zone, ProviderName(plugin.(caddy.Module))))

					if _, ok := updates[x]; !ok {
						updates[x] = make(map[string][]libdns.Record)
					}

					if _, ok := updates[x][zone]; !ok {
						updates[x][zone] = make([]libdns.Record, 0)
					}

					updates[x][zone] = append(updates[x][zone], libdns.Address{
						Name: libdns.RelativeName(hostname, zone),
						TTL:  (time.Minute * 5).Round(time.Second),
						IP:   ip,
					})

					continue hostnames
				}
			}

			h.logger.Debug(fmt.Sprintf("hostname %s is not supported by module %s", hostname, ProviderName(plugin.(caddy.Module))), zap.Strings("zones", zones[x]))
		}

		h.logger.Warn(fmt.Sprintf("hostname %s not supported by providers", hostname))

		(*result)[idx] = NoHost
	}

	return updates
}
