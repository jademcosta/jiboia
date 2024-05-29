package domain

import (
	"log/slog"
	"sync"

	"github.com/prometheus/client_golang/prometheus"
)

const COMPONENT_LABEL string = "component"

var ensureMetricRegisteringOnce sync.Once

type DataDropper interface {
	Drop([]byte)
}

type ObservableDataDropper struct {
	l              *slog.Logger
	componentOwner string
	counter        *prometheus.CounterVec
}

func NewObservableDataDropper(l *slog.Logger, metricRegistry *prometheus.Registry, owner string) DataDropper {
	dropCounter := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name:      "dropped_packages_total",
			Namespace: "jiboia",
			Help:      "How may data packages have been dropped",
		},
		[]string{COMPONENT_LABEL},
	)

	ensureMetricRegisteringOnce.Do(func() {
		metricRegistry.MustRegister(dropCounter)
	})

	return &ObservableDataDropper{
		l:              l,
		componentOwner: owner,
		counter:        dropCounter,
	}
}

func (dropper *ObservableDataDropper) Drop(data []byte) {
	dropper.counter.WithLabelValues(dropper.componentOwner).Inc()
	dropper.l.Warn("data has just been dropped", "size_in_bytes", len(data), "subject", dropper.componentOwner)
}
