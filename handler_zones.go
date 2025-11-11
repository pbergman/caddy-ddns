package dyndns_handler

import (
	"context"
	"fmt"
	"strings"

	"github.com/caddyserver/caddy/v2"
	"go.uber.org/zap"
)

func getAvailableZones(ctx context.Context, providers []Provider, lock WaitableLocker, logger *zap.Logger) [][]string {

	type job struct {
		idx      *int
		zones    *[]string
		provider Provider
	}

	var zones = make([][]string, len(providers))
	var work = make([]*job, len(providers))

	for i, c := 0, len(providers); i < c; i++ {
		zones[i] = make([]string, 0)
		work[i] = &job{
			idx:      &i,
			zones:    &zones[i],
			provider: providers[i],
		}
	}

	for idx, _ := range providers {

		lock.Lock()

		go func(x *job) {
			defer lock.Unlock()

			items, err := x.provider.ListZones(ctx)

			if err != nil {
				logger.Error(
					fmt.Sprintf("could not fetch zones: %s", err.Error()),
					zap.String("module", ProviderName(x.provider.(caddy.Module))),
					zap.Int("module idx", *x.idx),
				)
				return
			}

			for _, zone := range items {
				*x.zones = append(*x.zones, strings.TrimSuffix(zone.Name, "."))
			}

		}(work[idx])
	}

	lock.Wait()

	return zones
}
