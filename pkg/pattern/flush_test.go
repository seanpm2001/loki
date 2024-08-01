package pattern

import (
	"context"
	"math"
	"testing"
	"time"

	"github.com/go-kit/log"
	"github.com/grafana/dskit/flagext"
	"github.com/grafana/dskit/kv"
	"github.com/grafana/dskit/ring"
	ring_client "github.com/grafana/dskit/ring/client"
	"github.com/grafana/dskit/services"
	"github.com/grafana/dskit/user"
	"github.com/prometheus/prometheus/model/labels"
	"github.com/stretchr/testify/require"

	"github.com/grafana/loki/v3/pkg/logproto"
	"github.com/grafana/loki/v3/pkg/pattern/iter"

	"github.com/grafana/loki/pkg/push"
)

func TestSweepInstance(t *testing.T) {
	ringClient := &fakeRingClient{}
	ing, err := New(defaultIngesterTestConfig(t), ringClient, "foo", nil, log.NewNopLogger())
	require.NoError(t, err)
	defer services.StopAndAwaitTerminated(context.Background(), ing) //nolint:errcheck
	err = services.StartAndAwaitRunning(context.Background(), ing)
	require.NoError(t, err)

	lbs := labels.New(labels.Label{Name: "test", Value: "test"})
	ctx := user.InjectOrgID(context.Background(), "foo")
	_, err = ing.Push(ctx, &push.PushRequest{
		Streams: []push.Stream{
			{
				Labels: lbs.String(),
				Entries: []push.Entry{
					{
						Timestamp: time.Unix(20, 0),
						Line:      "ts=1 msg=hello",
					},
				},
			},
			{
				Labels: `{test="test",foo="bar"}`,
				Entries: []push.Entry{
					{
						Timestamp: time.Now(),
						Line:      "ts=1 msg=foo",
					},
				},
			},
		},
	})
	require.NoError(t, err)

	inst, _ := ing.getInstanceByID("foo")

	it, err := inst.Iterator(ctx, &logproto.QueryPatternsRequest{
		Query: `{test="test"}`,
		Start: time.Unix(0, 0),
		End:   time.Unix(0, math.MaxInt64),
	})
	require.NoError(t, err)
	res, err := iter.ReadAll(it)
	require.NoError(t, err)
	require.Equal(t, 2, len(res.Series))
	ing.sweepUsers(true, true)
	it, err = inst.Iterator(ctx, &logproto.QueryPatternsRequest{
		Query: `{test="test"}`,
		Start: time.Unix(0, 0),
		End:   time.Unix(0, math.MaxInt64),
	})
	require.NoError(t, err)
	res, err = iter.ReadAll(it)
	require.NoError(t, err)
	require.Equal(t, 1, len(res.Series))
}

func defaultIngesterTestConfig(t testing.TB) Config {
	kvClient, err := kv.NewClient(kv.Config{Store: "inmemory"}, ring.GetCodec(), nil, log.NewNopLogger())
	require.NoError(t, err)

	cfg := Config{}
	flagext.DefaultValues(&cfg)
	cfg.FlushCheckPeriod = 99999 * time.Hour
	cfg.ConcurrentFlushes = 1
	cfg.LifecyclerConfig.RingConfig.KVStore.Mock = kvClient
	cfg.LifecyclerConfig.NumTokens = 1
	cfg.LifecyclerConfig.ListenPort = 0
	cfg.LifecyclerConfig.Addr = "localhost"
	cfg.LifecyclerConfig.ID = "localhost"
	cfg.LifecyclerConfig.FinalSleep = 0
	cfg.LifecyclerConfig.MinReadyDuration = 0

	return cfg
}

type fakeRingClient struct{}

func (f *fakeRingClient) Pool() *ring_client.Pool {
	panic("not implemented")
}

func (f *fakeRingClient) StartAsync(_ context.Context) error {
	panic("not implemented")
}

func (f *fakeRingClient) AwaitRunning(_ context.Context) error {
	panic("not implemented")
}

func (f *fakeRingClient) StopAsync() {
	panic("not implemented")
}

func (f *fakeRingClient) AwaitTerminated(_ context.Context) error {
	panic("not implemented")
}

func (f *fakeRingClient) FailureCase() error {
	panic("not implemented")
}

func (f *fakeRingClient) State() services.State {
	panic("not implemented")
}

func (f *fakeRingClient) AddListener(_ services.Listener) {
	panic("not implemented")
}

func (f *fakeRingClient) Ring() ring.ReadRing {
	return &fakeRing{}
}

type fakeRing struct{}

// InstancesWithTokensCount returns the number of instances in the ring that have tokens.
func (f *fakeRing) InstancesWithTokensCount() int {
	panic("not implemented") // TODO: Implement
}

// InstancesInZoneCount returns the number of instances in the ring that are registered in given zone.
func (f *fakeRing) InstancesInZoneCount(_ string) int {
	panic("not implemented") // TODO: Implement
}

// InstancesWithTokensInZoneCount returns the number of instances in the ring that are registered in given zone and have tokens.
func (f *fakeRing) InstancesWithTokensInZoneCount(_ string) int {
	panic("not implemented") // TODO: Implement
}

// ZonesCount returns the number of zones for which there's at least 1 instance registered in the ring.
func (f *fakeRing) ZonesCount() int {
	panic("not implemented") // TODO: Implement
}

func (f *fakeRing) Get(
	_ uint32,
	_ ring.Operation,
	_ []ring.InstanceDesc,
	_ []string,
	_ []string,
) (ring.ReplicationSet, error) {
	panic("not implemented")
}

func (f *fakeRing) GetAllHealthy(_ ring.Operation) (ring.ReplicationSet, error) {
	return ring.ReplicationSet{}, nil
}

func (f *fakeRing) GetReplicationSetForOperation(_ ring.Operation) (ring.ReplicationSet, error) {
	return ring.ReplicationSet{}, nil
}

func (f *fakeRing) ReplicationFactor() int {
	panic("not implemented")
}

func (f *fakeRing) InstancesCount() int {
	panic("not implemented")
}

func (f *fakeRing) ShuffleShard(_ string, _ int) ring.ReadRing {
	panic("not implemented")
}

func (f *fakeRing) GetInstanceState(_ string) (ring.InstanceState, error) {
	panic("not implemented")
}

func (f *fakeRing) ShuffleShardWithLookback(
	_ string,
	_ int,
	_ time.Duration,
	_ time.Time,
) ring.ReadRing {
	panic("not implemented")
}

func (f *fakeRing) HasInstance(_ string) bool {
	panic("not implemented")
}

func (f *fakeRing) CleanupShuffleShardCache(_ string) {
	panic("not implemented")
}

func (f *fakeRing) GetTokenRangesForInstance(_ string) (ring.TokenRanges, error) {
	panic("not implemented")
}
