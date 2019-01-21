package main

import (
	"reflect"
	"strings"

	"github.com/gluster/gluster-prometheus/pkg/glusterutils"
	"github.com/prometheus/client_golang/prometheus"
)

// GMetricProvider inerface provides gluster related metrics
type GMetricProvider interface {
	ProvideMetricName() string
	ProvideDescs() []*prometheus.Desc
	ProvideMetrics() []prometheus.Metric
	ProvideCollector() prometheus.Collector
}

// GMetricGenerator interface generates gluster metrics
type GMetricGenerator interface {
	GenerateMetrics(glusterutils.GInterface) error
}

// GMetricGenProvider interface implements both
// GMetricProvider and GMetricGenerator interfaces
type GMetricGenProvider interface {
	GMetricProvider
	GMetricGenerator
}

// Reseter interface provides a way to reset data
type Reseter interface {
	Reset()
}

// GCollector interface provides a custom prometheus collector for gluster
type GCollector interface {
	GetName() string
	Enabled() bool
	Enable(bool)
	prometheus.Collector
}

// MLabels is an alias for MetricLabel array
type MLabels []MetricLabel

// Names returns an array of names in the labels
func (mLbls MLabels) Names() (names []string) {
	for _, eachMLbl := range mLbls {
		names = append(names, eachMLbl.Name)
	}
	return
}

// String returns the string representation of labels.
// It creates a unique name out of all the 'Name' field of each Label
func (mLbls MLabels) String() string {
	// a double '_' may help us (in future)
	// to retrieve MetricLabel names from a created string
	return strings.Join(mLbls.Names(), "__")
}

// ValuesInOrder returns an ordered list of values in 'pl'
// This is necessary as the values' order is expected,
// as same as its been passed to the 'Desc'
func (mLbls MLabels) ValuesInOrder(pl prometheus.Labels) (vlArr []string) {
	for _, eachN := range mLbls.Names() {
		vlArr = append(vlArr, pl[eachN])
	}
	return
}

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

// FullName method returns the 'Namespace'_'Name' string
func (m Metric) FullName() string {
	return m.Namespace + "_" + m.Name
}

// LabelNames returns list of Prometheus labels
func (m Metric) LabelNames() []string {
	return MLabels(m.Labels).Names()
}

// convertIFaceToCollectors returns 'prometheus.Collector' array from a single interface
// courtesy: StackOverflow
// https://stackoverflow.com/questions/12753805/type-converting-slices-of-interfaces-in-go
func convertIFaceToCollectors(iFace interface{}) (colArr []prometheus.Collector) {
	s := reflect.ValueOf(iFace)
	// check the value we received is fo type 'Slice'
	if s.Kind() != reflect.Slice {
		return
	}
	for i := 0; i < s.Len(); i++ {
		if col, ok := s.Index(i).Interface().(prometheus.Collector); ok {
			colArr = append(colArr, col)
		}
	}
	return
}

// newPrometheusGaugeVec is wrapper around prometheus.NewGaugeVec. Additionally
// it registers with global map which can be used for documentation,
// listing supported metrics via CLI etc.
//
// 'gaugeVecArrPtr' helps to collect all the 'GaugeVec' pertaining to a metric group.
// Ignored if 'nil'
func newPrometheusGaugeVec(m Metric, gaugeVecArrPtr *[]*prometheus.GaugeVec) *prometheus.GaugeVec {
	gaugeVec := prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: m.Namespace,
			Name:      m.Name,
			Help:      m.Help,
		},
		m.LabelNames(),
	)

	if gaugeVecArrPtr != nil {
		*gaugeVecArrPtr = append(*gaugeVecArrPtr, gaugeVec)
	}

	// Add the metric to the global queue
	metrics = append(metrics, m)

	return gaugeVec
}

// newPrometheusDesc method creates a new 'prometheus.Desc'.
// Optionally it can capture the 'Desc' into an array.
// Always remember to pass an address of a slice to 'descArrPtr'.
// Pass 'nil' to ignore.
func newPrometheusDesc(m Metric, descArrPtr *[]*prometheus.Desc) *prometheus.Desc {
	// Add the metric to the global queue
	metrics = append(metrics, m)

	newDesc := prometheus.NewDesc(
		prometheus.BuildFQName(m.Namespace, "", m.Name),
		m.Help,
		m.LabelNames(),
		nil,
	)
	if descArrPtr != nil {
		*descArrPtr = append(*descArrPtr, newDesc)
	}
	return newDesc
}

var metrics []Metric
