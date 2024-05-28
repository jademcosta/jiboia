package circuitbreaker

import (
	"sync"

	"github.com/jademcosta/jiboia/pkg/logger"
	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/zap"
)

const (
	FLOW_METRIC_KEY string = "flow"
	NAME_METRIC_KEY string = "name"

	cbClosed = 0.0
	cbOpen   = 1.0
)

var ensureMetricRegisteringOnce sync.Once

var openCBGauge *prometheus.GaugeVec
var openCBTotal *prometheus.CounterVec

type CBObservability struct {
	name string
	flow string
	log  *zap.SugaredLogger
}

func NewCBObservability(
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

		openCBTotal = prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: "jiboia",
				Name:      "circuitbreaker_open_total",
				Help:      "How many times have circuitbreaker opened",
			},
			[]string{FLOW_METRIC_KEY, NAME_METRIC_KEY},
		)

		registry.MustRegister(openCBGauge, openCBTotal)
	})

	return &CBObservability{
		name: name,
		flow: flow,
		log:  log.With(logger.FLOW_KEY, flow, NAME_METRIC_KEY, name),
	}
}

func (cbO11y *CBObservability) SetCBClosed() {
	openCBGauge.WithLabelValues(cbO11y.flow, cbO11y.name).Set(cbClosed)
	cbO11y.log.Info("circuitbreaker is closed")
}

func (cbO11y *CBObservability) SetCBOpen() {
	openCBGauge.WithLabelValues(cbO11y.flow, cbO11y.name).Set(cbOpen)
	openCBTotal.WithLabelValues(cbO11y.flow, cbO11y.name).Inc()
	cbO11y.log.Warn("circuitbreaker is open")
}
