package main

import (
	"github.com/prometheus/client_golang/prometheus"
)

// MetricLabel represents Prometheus Label
type MetricLabel struct {
	Name string
	Help string
}

// Metric represents Prometheus metric
type Metric struct {
	Name      string
	Help      string
	LongHelp  string
	Namespace string
	Disabled  bool
	Labels    []MetricLabel
}

// LabelNames returns list of Prometheus labels
func (m *Metric) LabelNames() []string {
	out := make([]string, len(m.Labels))
	for idx, lbl := range m.Labels {
		out[idx] = lbl.Name
	}
	return out
}

// newPrometheusGaugeVec is wrapper around prometheus.NewGaugeVec. Additionally
// it registers with global map which can be used for documentation,
// listing supported metrics via CLI etc.
func newPrometheusGaugeVec(m Metric) *prometheus.GaugeVec {
	gaugeVec := prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: m.Namespace,
			Name:      m.Name,
			Help:      m.Help,
		},
		m.LabelNames(),
	)

	// Add the metric to the global queue
	metrics = append(metrics, m)

	return gaugeVec
}

func newPrometheusDesc(m Metric) *prometheus.Desc {
	// Add the metric to the global queue
	metrics = append(metrics, m)

	return prometheus.NewDesc(
		prometheus.BuildFQName(m.Namespace, "", m.Name),
		m.Help,
		m.LabelNames(),
		nil,
	)
}

var metrics []Metric
