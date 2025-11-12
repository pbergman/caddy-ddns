package dyndns_handler

import (
	"io"
	"net/netip"

	"github.com/libdns/libdns"
	"go.uber.org/zap"
)

type ReturnCode string

// https://help.dyn.com/return-codes.html
const (
	Good                        ReturnCode = "good"
	NoChange                    ReturnCode = "nochg"
	NotFullyQualifiedDomainName ReturnCode = "notfqdn"
	DNSError                    ReturnCode = "dnserr"
	NoHost                      ReturnCode = "nohost"
	BadAuthentication           ReturnCode = "badauth"
)

func (h *Handler) writeReturnCode(writer io.Writer, ip *netip.Addr, hosts []string, codes ...ReturnCode) error {

	var buf = make([]byte, 0)
	var size = len(codes)
	var fields = make([]zap.Field, size)

	if size == 0 {
		return nil
	}

	for i, c := 0, size; i < c; i++ {

		var value = string(codes[i])

		if (codes[i] == Good || codes[i] == NoChange) && ip != nil {
			value += " " + ip.String()
		}

		buf = append(buf, value...)

		if hosts == nil || len(fields) != size {
			fields[i] = zap.String("code", value)
		} else {
			fields[i] = zap.String(hosts[i], value)
		}

		if size-1 > i {
			buf = append(buf, "\n"...)
		}

	}

	h.logger.Info("ddns update response", fields...)

	_, err := writer.Write(buf)

	return err
}

func (h *Handler) setReturnCodes(result []ReturnCode, value ReturnCode) []ReturnCode {

	for i, c := 0, len(result); i < c; i++ {
		result[i] = value
	}

	return result
}

func (h *Handler) setReturnCodesForItems(result *[]ReturnCode, items []libdns.Record, value ReturnCode, zone string, hosts []string) {
	for _, item := range items {
		if x := getHostIdx(hosts, item.RR().Name, zone); x != -1 {
			(*result)[x] = value
		}
	}
}
