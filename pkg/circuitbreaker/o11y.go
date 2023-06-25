package circuitbreaker

import (
	"sync"

	"github.com/jademcosta/jiboia/pkg/logger"
	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/zap"
)

const FLOW_METRIC_KEY string = "flow"
const NAME_METRIC_KEY string = "name"

var ensureMetricRegisteringOnce sync.Once

var openCBGauge *prometheus.GaugeVec

type CBObservability struct {
	name string
	flow string
	log  *zap.SugaredLogger
}

func NewObservability(
	registry *prometheus.Registry,
	log *zap.SugaredLogger,
	name string,
	flow string,
) *CBObservability {

	ensureMetricRegisteringOnce.Do(func() {
		openCBGauge = prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace: "jiboia",
				Name:      "circuitbreaker_open",
				Help:      "Value is 1 when the circuit breaker is open",
			},
			[]string{FLOW_METRIC_KEY, NAME_METRIC_KEY},
		)

		registry.MustRegister(openCBGauge)
	})

	return &CBObservability{
		name: name,
		flow: flow,
		log:  log.With(logger.FLOW_KEY, flow, NAME_METRIC_KEY, name),
	}
}

func (cbO11y *CBObservability) cbClosed() {
	openCBGauge.WithLabelValues(cbO11y.flow, cbO11y.name).Set(0.0)
	cbO11y.log.Info("circuitbreaker is closed")
}

func (cbO11y *CBObservability) cbOpen() {
	openCBGauge.WithLabelValues(cbO11y.flow, cbO11y.name).Set(1.0)
	cbO11y.log.Warn("circuitbreaker is open")
}
