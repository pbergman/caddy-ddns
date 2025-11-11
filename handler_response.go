package dyndns_handler

import (
	"io"
	"net/netip"
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

func WriterReturnCode(writer io.Writer, ip *netip.Addr, codes ...ReturnCode) error {

	var buf = make([]byte, 0)
	var size = len(codes)

	if size == 0 {
		return nil
	}

	for i, c := 0, size; i < c; i++ {

		var suffix string

		if size-1 > i {
			suffix = "\n"
		}

		if (codes[i] == Good || codes[i] == NoChange) && ip != nil {
			suffix = " " + ip.String() + suffix
		}

		buf = append(buf, string(codes[i])+suffix...)
	}

	_, err := writer.Write(buf)

	return err
}
