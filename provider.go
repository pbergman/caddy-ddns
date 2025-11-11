package dyndns_handler

import (
	"github.com/libdns/libdns"
)

type Provider interface {
	BaseProvider
	libdns.ZoneLister
}

type BaseProvider interface {
	libdns.RecordSetter
	libdns.RecordDeleter
	libdns.RecordAppender
	libdns.RecordGetter
}
