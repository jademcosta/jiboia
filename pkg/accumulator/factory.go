package accumulator

import (
	"log/slog"
	"time"

	"github.com/jademcosta/jiboia/pkg/circuitbreaker"
	"github.com/jademcosta/jiboia/pkg/config"
	"github.com/jademcosta/jiboia/pkg/domain"
	"github.com/jademcosta/jiboia/pkg/domain/flow"
	"github.com/prometheus/client_golang/prometheus"
)

// TODO: add tests
func From(
	conf config.AccumulatorConfig,
	flowName string,
	logg *slog.Logger,
	limitOfBytes int,
	next domain.DataEnqueuer,
	cb circuitbreaker.CircuitBreaker,
	metricRegistry *prometheus.Registry,
	currentTimeProvider func() time.Time,
	forceFlushAfter time.Duration,
) flow.DataFlowRunnable {

	if conf.HasForceFlushPeriod() {
		return NewAccumulatorByTimeAndSize(flowName, logg, limitOfBytes, []byte(conf.Separator),
			conf.QueueCapacity, next, cb, metricRegistry, currentTimeProvider, forceFlushAfter,
		)
	}

	return NewAccumulatorBySize(flowName, logg, limitOfBytes, []byte(conf.Separator), conf.QueueCapacity,
		next, cb, metricRegistry, currentTimeProvider, forceFlushAfter)
}
