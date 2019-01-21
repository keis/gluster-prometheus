package main

import (
	"flag"
	"fmt"
	"net/http"
	"os"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/gluster/gluster-prometheus/gluster-exporter/conf"
	"github.com/gluster/gluster-prometheus/pkg/glusterutils"
	"github.com/gluster/gluster-prometheus/pkg/logging"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	log "github.com/sirupsen/logrus"
)

// Below variables are set as flags during build time. The current
// values are just placeholders
var (
	exporterVersion         = ""
	defaultGlusterd1Workdir = ""
	defaultGlusterd2Workdir = ""
	defaultConfFile         = ""
)

var (
	showVersion                   = flag.Bool("version", false, "Show the version information")
	docgen                        = flag.Bool("docgen", false, "Generate exported metrics documentation in Asciidoc format")
	config                        = flag.String("config", defaultConfFile, "Config file path")
	defaultInterval time.Duration = 5
	glusterConfig   glusterutils.Config
)

// gMetricGenProvider implements both
// 'GMetricProvider' and 'GMetricGenerator' interfaces.
// This is a general purpose metrics generator and provider,
// but individual metrics providers can implement their own version of
// 'GMetricGenProvider' interface
type gMetricGenProvider struct {
	name     string
	descArr  []*prometheus.Desc
	metrics  []prometheus.Metric
	mGenFunc func(glusterutils.GInterface) ([]prometheus.Metric, error)
}

func newGMetricGenProvider(name string,
	genFunc func(glusterutils.GInterface) ([]prometheus.Metric, error),
	descArr []*prometheus.Desc) *gMetricGenProvider {
	gmp := &gMetricGenProvider{name: name, descArr: descArr, mGenFunc: genFunc}
	return gmp
}

func (gmGP *gMetricGenProvider) ProvideMetricName() string {
	return gmGP.name
}

func (gmGP *gMetricGenProvider) ProvideDescs() []*prometheus.Desc {
	return gmGP.descArr
}

func (gmGP *gMetricGenProvider) ProvideMetrics() []prometheus.Metric {
	return gmGP.metrics
}

func (gmGP *gMetricGenProvider) ProvideCollector() prometheus.Collector {
	return nil
}

func (gmGP *gMetricGenProvider) GenerateMetrics(gi glusterutils.GInterface) (err error) {
	// no error is returned if the function is nil
	if gmGP.mGenFunc != nil {
		gmGP.metrics, err = gmGP.mGenFunc(gi)
	}
	return
}

// gMetricCollectGenProvider embeds 'gMetricGenProvider'
// A general purpose metrics generator which provides it's own collector
type gMetricCollectGenProvider struct {
	*gMetricGenProvider
	mGenFunc func(glusterutils.GInterface) error
	colArr   []prometheus.Collector
}

func newGMetricCollectGenProvider(name string,
	genFunc func(glusterutils.GInterface) error,
	colArrI interface{}) *gMetricCollectGenProvider {
	// newGMCGP := &gMetricCollectGenProvider{&gMetricGenProvider{name: name}}
	gmp := &gMetricCollectGenProvider{
		mGenFunc:           genFunc,
		gMetricGenProvider: &gMetricGenProvider{name: name},
	}
	gmp.colArr = convertIFaceToCollectors(colArrI)
	return gmp
}

func (gmCGP *gMetricCollectGenProvider) Describe(ch chan<- *prometheus.Desc) {
	for _, eachGV := range gmCGP.colArr {
		eachGV.Describe(ch)
	}
}

func (gmCGP *gMetricCollectGenProvider) Collect(ch chan<- prometheus.Metric) {
	for _, eachGV := range gmCGP.colArr {
		eachGV.Collect(ch)
	}
}

func (gmCGP *gMetricCollectGenProvider) ProvideCollector() prometheus.Collector {
	return gmCGP
}

func (gmCGP *gMetricCollectGenProvider) GenerateMetrics(gi glusterutils.GInterface) (err error) {
	// if possible, reset
	for _, eachGV := range gmCGP.colArr {
		if r, ok := eachGV.(Reseter); ok {
			r.Reset()
		}
	}
	// if there is no function associated, return silently
	if gmCGP.mGenFunc == nil {
		return
	}
	return gmCGP.mGenFunc(gi)
}

// gCollector implements 'GCollector' interface
type gCollector struct {
	gMProvider  GMetricProvider
	enabled     bool
	collectLock sync.Mutex
}

// newGCollector method returns a new gluster collector object
func newGCollector(gmp GMetricProvider, enabled bool) GCollector {
	return &gCollector{gMProvider: gmp, enabled: enabled}
}

func (gCol *gCollector) GetName() string {
	return gCol.gMProvider.ProvideMetricName()
}

func (gCol *gCollector) Enabled() bool {
	return gCol.enabled
}

func (gCol *gCollector) Enable(enable bool) {
	gCol.enabled = enable
}

// Describe method implements prometheus.Collector.Describe method
func (gCol *gCollector) Describe(ch chan<- *prometheus.Desc) {
	// if not enabled, do nothing
	if !gCol.Enabled() {
		return
	}
	// if gluster metrics provider provides it's own collector, use it
	if col := gCol.gMProvider.ProvideCollector(); col != nil {
		col.Describe(ch)
		return
	}
	for _, desc := range gCol.gMProvider.ProvideDescs() {
		ch <- desc
	}
}

// Collect method implements prometheus.Collector.Collect method
// Excerpt from prometheus.Collector interface:
// This method may be called concurrently and must therefore be
// implemented in a concurrency safe way.
func (gCol *gCollector) Collect(ch chan<- prometheus.Metric) {
	// if not enabled, do nothing
	if !gCol.Enabled() {
		return
	}
	// if gluster metrics provider provides it's own collector, use it
	if col := gCol.gMProvider.ProvideCollector(); col != nil {
		col.Collect(ch)
		return
	}
	gCol.collectLock.Lock()
	defer gCol.collectLock.Unlock()
	allMs := gCol.gMProvider.ProvideMetrics()
	for _, m := range allMs {
		ch <- m
	}
}

var gmGenProviders []GMetricGenProvider

func registerNewGMetricGenProvider(gmGP GMetricGenProvider) {
	gmGenProviders = append(gmGenProviders, gmGP)
}

func dumpVersionInfo() {
	fmt.Printf("version   : %s\n", exporterVersion)
	fmt.Printf("go version: %s\n", runtime.Version())
	fmt.Printf("go OS/arch: %s/%s\n", runtime.GOOS, runtime.GOARCH)
}

func getDefaultGlusterdDir(mgmt string) string {
	if mgmt == glusterutils.MgmtGlusterd2 {
		return defaultGlusterd2Workdir
	}
	return defaultGlusterd1Workdir
}

func main() {
	// Init logger with stderr, will be reinitialized later
	if err := logging.Init("", "-", "info"); err != nil {
		log.Fatal("Init logging failed for stderr")
	}

	flag.Parse()

	if *docgen {
		generateMetricsDoc()
		return
	}

	if *showVersion {
		dumpVersionInfo()
		return
	}

	var gluster glusterutils.GInterface
	exporterConf, err := conf.LoadConfig(*config)
	if err != nil {
		log.WithError(err).Fatal("Loading global config failed")
	}

	// Create Log dir
	err = os.MkdirAll(exporterConf.GlobalConf.LogDir, 0750)
	if err != nil {
		log.WithError(err).WithField("logdir", exporterConf.GlobalConf.LogDir).
			Fatal("Failed to create log directory")
	}

	if err := logging.Init(exporterConf.GlobalConf.LogDir, exporterConf.GlobalConf.LogFile, exporterConf.GlobalConf.LogLevel); err != nil {
		log.WithError(err).Fatal("Failed to initialize logging")
	}

	// Set the Gluster Configurations used in glusterutils
	glusterConfig.GlusterMgmt = "glusterd"
	if exporterConf.GlobalConf.GlusterMgmt != "" {
		glusterConfig.GlusterMgmt = exporterConf.GlobalConf.GlusterMgmt
		if exporterConf.GlobalConf.GlusterMgmt == "glusterd2" {
			if exporterConf.GlobalConf.GD2RESTEndpoint != "" {
				glusterConfig.Glusterd2Endpoint = exporterConf.GlobalConf.GD2RESTEndpoint
			}
		}
	}

	// If GD2_ENDPOINTS env variable is set, use that info
	// for making REST API calls
	if endpoint := os.Getenv("GD2_ENDPOINTS"); endpoint != "" {
		glusterConfig.Glusterd2Endpoint = strings.Split(endpoint, ",")[0]
	}

	glusterConfig.GlusterdWorkdir = getDefaultGlusterdDir(glusterConfig.GlusterMgmt)
	if exporterConf.GlobalConf.GlusterdDir != "" {
		glusterConfig.GlusterdWorkdir = exporterConf.GlobalConf.GlusterdDir
	}
	gluster = glusterutils.MakeGluster(&glusterConfig, exporterConf)

	// start := time.Now()

	for _, m := range gmGenProviders {
		if collectorConf, ok := exporterConf.CollectorsConf[m.ProvideMetricName()]; ok {
			if !collectorConf.Disabled {
				go func(m GMetricGenProvider, gi glusterutils.GInterface) {
					// create a gluster collector for this metrics
					gCol := newGCollector(m, true)
					prometheus.MustRegister(gCol)
					for {
						err := m.GenerateMetrics(gi)
						interval := defaultInterval
						if collectorConf.SyncInterval > 0 {
							interval = time.Duration(collectorConf.SyncInterval)
						}
						if err != nil {
							log.WithError(err).WithFields(log.Fields{
								"name": m.ProvideMetricName(),
							}).Error("failed to export metric")
						}
						time.Sleep(time.Second * interval)
					}
				}(m, gluster)
			}
		}
	}

	if len(gmGenProviders) == 0 {
		_, _ = fmt.Fprintf(os.Stderr, "No Metrics registered, Exiting..\n")
		os.Exit(1)
	}

	metricsPath := exporterConf.GlobalConf.MetricsPath
	port := exporterConf.GlobalConf.Port
	http.Handle(metricsPath, promhttp.Handler())
	if err := http.ListenAndServe(fmt.Sprintf(":%d", port), nil); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "Failed to run exporter\nError: %s", err)
		log.WithError(err).Fatal("Failed to run exporter")
	}
}
