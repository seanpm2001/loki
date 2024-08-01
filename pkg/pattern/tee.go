package pattern

import (
	"context"
	"errors"

	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
	"github.com/grafana/dskit/ring"
	"github.com/grafana/dskit/user"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"

	"github.com/grafana/loki/v3/pkg/distributor"
	"github.com/grafana/loki/v3/pkg/logproto"
)

type Tee struct {
	cfg        Config
	logger     log.Logger
	ringClient *RingClient

	ingesterAppends         *prometheus.CounterVec
	fallbackIngesterAppends *prometheus.CounterVec
}

func NewTee(
	cfg Config,
	ringClient *RingClient,
	metricsNamespace string,
	registerer prometheus.Registerer,
	logger log.Logger,
) (*Tee, error) {
	registerer = prometheus.WrapRegistererWithPrefix(metricsNamespace+"_", registerer)

	t := &Tee{
		logger: log.With(logger, "component", "pattern-tee"),
		ingesterAppends: promauto.With(registerer).NewCounterVec(prometheus.CounterOpts{
			Name: "pattern_ingester_appends_total",
			Help: "The total number of batch appends of owned streams sent to pattern ingesters.",
		}, []string{"ingester", "status"}),
		fallbackIngesterAppends: promauto.With(registerer).NewCounterVec(prometheus.CounterOpts{
			Name: "pattern_ingester_fallback_appends_total",
			Help: "The total number of batch appends sent to fallback pattern ingesters, for not owned streams.",
		}, []string{"ingester", "status"}),
		cfg:        cfg,
		ringClient: ringClient,
	}

	return t, nil
}

// Duplicate Implements distributor.Tee which is used to tee distributor requests to pattern ingesters.
func (t *Tee) Duplicate(tenant string, streams []distributor.KeyedStream) {
	for idx := range streams {
		go func(stream distributor.KeyedStream) {
			if err := t.sendStream(tenant, stream); err != nil {
				level.Error(t.logger).Log("msg", "failed to send stream to pattern ingester", "err", err)
			}
		}(streams[idx])
	}
}

func (t *Tee) sendStream(tenant string, stream distributor.KeyedStream) error {
	err := t.sendOwnedStream(tenant, stream)
	if err == nil {
		return nil
	}

	// Pattern ingesters serve 2 functions, processing patterns and aggregating metrics.
	// Only owned streams are processed for patterns, however any pattern ingester can
	// aggregate metrics for any stream. Therefore, if we can't send the owned stream,
	// try to send it to any pattern ingester so we at least capture the metrics.
	replicationSet, err := t.ringClient.ring.GetAllHealthy(ring.Read)
	if replicationSet.Instances == nil {
		return errors.New("no instances found")
	}

	for _, instance := range replicationSet.Instances {
		addr := instance.Addr
		client, err := t.ringClient.pool.GetClientFor(addr)
		if err != nil {
			req := &logproto.PushRequest{
				Streams: []logproto.Stream{
					stream.Stream,
				},
			}

			ctx, cancel := context.WithTimeout(
				user.InjectOrgID(context.Background(), tenant),
				t.cfg.ClientConfig.RemoteTimeout,
			)
			defer cancel()
			_, err = client.(logproto.PatternClient).Push(ctx, req)
			if err != nil {
				t.fallbackIngesterAppends.WithLabelValues(addr, "fail").Inc()
				continue
			}
			t.fallbackIngesterAppends.WithLabelValues(addr, "success").Inc()
			// bail after any success to prevent sending more than one
			return nil
		}
	}

	return err
}

func (t *Tee) sendOwnedStream(tenant string, stream distributor.KeyedStream) error {
	var descs [1]ring.InstanceDesc
	replicationSet, err := t.ringClient.ring.Get(stream.HashKey, ring.WriteNoExtend, descs[:0], nil, nil)
	if err != nil {
		return err
	}
	if replicationSet.Instances == nil {
		return errors.New("no instances found")
	}
	addr := replicationSet.Instances[0].Addr
	client, err := t.ringClient.pool.GetClientFor(addr)
	if err != nil {
		return err
	}
	req := &logproto.PushRequest{
		Streams: []logproto.Stream{
			stream.Stream,
		},
	}

	ctx, cancel := context.WithTimeout(user.InjectOrgID(context.Background(), tenant), t.cfg.ClientConfig.RemoteTimeout)
	defer cancel()
	_, err = client.(logproto.PatternClient).Push(ctx, req)
	if err != nil {
		t.ingesterAppends.WithLabelValues(addr, "fail").Inc()
		return err
	}
	t.ingesterAppends.WithLabelValues(addr, "success").Inc()
	return nil
}
