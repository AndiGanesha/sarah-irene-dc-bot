package metrics

import "github.com/prometheus/client_golang/prometheus"

type Metrics struct {
	Registry *prometheus.Registry

	EventsTotal *prometheus.CounterVec
	DMSentTotal *prometheus.CounterVec
	DMErrorsTotal *prometheus.CounterVec
	QueueDepth prometheus.Gauge
}

func New() *Metrics {
	reg := prometheus.NewRegistry()

	events := prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "vc_sentry_events_total",
		Help: "Incoming Discord events by type",
	}, []string{"type"})

	dmSent := prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "vc_sentry_dm_sent_total",
		Help: "DMs sent by reason",
	}, []string{"reason"})

	dmErr := prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "vc_sentry_dm_errors_total",
		Help: "DM errors by reason",
	}, []string{"reason"})

	qDepth := prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "vc_sentry_queue_depth",
		Help: "Depth of DM job queue",
	})

	reg.MustRegister(events, dmSent, dmErr, qDepth)

	return &Metrics{Registry: reg, EventsTotal: events, DMSentTotal: dmSent, DMErrorsTotal: dmErr, QueueDepth: qDepth}
}
