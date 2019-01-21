package main

import (
	"io/ioutil"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/gluster/gluster-prometheus/pkg/glusterutils"

	"github.com/prometheus/client_golang/prometheus"
	log "github.com/sirupsen/logrus"
)

var (
	glusterProcs = []string{
		"glusterd",
		"glusterfsd",
		"glusterd2",
		// TODO: Add more processes
	}

	labels = []MetricLabel{
		{
			Name: "volume",
			Help: "Volume Name",
		},
		{
			Name: "peerid",
			Help: "Peer ID",
		},
		{
			Name: "brick_path",
			Help: "Brick Path",
		},
		{
			Name: "name",
			Help: "Name of the Gluster process(Ex: `glusterfsd`, `glusterd` etc)",
		},
	}

	// psDescs collects all the 'Desc'-s for ps metrics
	psDescs []*prometheus.Desc

	glusterCPUPercentage = newPrometheusDesc(Metric{
		Namespace: "gluster",
		Name:      "cpu_percentage",
		Help:      "CPU Percentage used by Gluster processes",
		LongHelp:  "CPU percentage of Gluster process. One metric will be exposed for each process. Note: values of labels will be empty if not applicable to that process. For example, glusterd process will not have labels for volume or brick_path. It is the CPU time used divided by the time the process has been running (cputime/realtime ratio), expressed as a percentage.",
		Labels:    labels,
	}, &psDescs)

	glusterMemoryPercentage = newPrometheusDesc(Metric{
		Namespace: "gluster",
		Name:      "memory_percentage",
		Help:      "Memory Percentage used by Gluster processes",
		LongHelp:  "Memory percentage of Gluster process. One metric will be exposed for each process. Note: values of labels will be empty if not applicable to that process. For example, glusterd process will not have labels for volume or brick_path. It is the ratio of the process's resident set size to the physical memory on the machine, expressed as a percentage",
		Labels:    labels,
	}, &psDescs)

	glusterResidentMemory = newPrometheusDesc(Metric{
		Namespace: "gluster",
		Name:      "resident_memory_bytes",
		Help:      "Resident Memory of Gluster processes in bytes",
		LongHelp:  "Resident Memory of Gluster process in bytes. One metric will be exposed for each process. Note: values of labels will be empty if not applicable to that process. For example, glusterd process will not have labels for volume or brick_path.",
		Labels:    labels,
	}, &psDescs)

	glusterVirtualMemory = newPrometheusDesc(Metric{
		Namespace: "gluster",
		Name:      "virtual_memory_bytes",
		Help:      "Virtual Memory of Gluster processes in bytes",
		LongHelp:  "Virtual Memory of Gluster process in bytes. One metric will be exposed for each process. Note: values of labels will be empty if not applicable to that process. For example, glusterd process will not have labels for volume or brick_path.",
		Labels:    labels,
	}, &psDescs)

	glusterElapsedTime = newPrometheusDesc(Metric{
		Namespace: "gluster",
		Name:      "elapsed_time_seconds",
		Help:      "Elapsed Time of Gluster processes in seconds",
		LongHelp:  "Elapsed Time or Uptime of Gluster processes in seconds. One metric will be exposed for each process. Note: values of labels will be empty if not applicable to that process. For example, glusterd process will not have labels for volume or brick_path.",
		Labels:    labels,
	}, &psDescs)
)

func getCmdLine(pid string) ([]string, error) {
	var args []string

	out, err := ioutil.ReadFile(filepath.Clean("/proc/" + pid + "/cmdline"))
	if err != nil {
		return args, err
	}

	return strings.Split(strings.Trim(string(out), "\x00"), "\x00"), nil
}

func getGlusterdLabels(peerID, cmd string, args []string) prometheus.Labels {
	return prometheus.Labels{
		"name":       cmd,
		"volume":     "",
		"peerid":     peerID,
		"brick_path": "",
	}
}

func getGlusterFsdLabels(peerID, cmd string, args []string) prometheus.Labels {
	bpath := ""
	volume := ""

	prevArg := ""

	for _, a := range args {
		if prevArg == "--brick-name" {
			bpath = a
		} else if prevArg == "--volfile-id" {
			volume = strings.Split(a, ".")[0]
		}
		prevArg = a
	}

	return prometheus.Labels{
		"name":       cmd,
		"volume":     volume,
		"peerid":     peerID,
		"brick_path": bpath,
	}
}

func getUnknownLabels(peerID, cmd string, args []string) prometheus.Labels {
	return prometheus.Labels{
		"name":       cmd,
		"volume":     "",
		"peerid":     peerID,
		"brick_path": "",
	}
}

func ps(gluster glusterutils.GInterface) ([]prometheus.Metric, error) {
	args := []string{
		"--no-header", // No header in the output
		"-ww",         // To set unlimited width to avoid crop
		"-o",          // Output Format
		"pid,pcpu,pmem,rsz,vsz,etimes,comm",
		"-C",
		strings.Join(glusterProcs, ","),
	}

	out, err := exec.Command("ps", args...).Output()

	if err != nil {
		// Return without exporting metrics in this cycle
		return nil, err
	}

	peerID, err := gluster.LocalPeerID()
	if err != nil {
		return nil, err
	}

	var metricArr []prometheus.Metric
	for _, line := range strings.Split(string(out), "\n") {
		// Sample data:
		// 6959  0.0  0.6 12840 713660  504076 glusterfs
		lineDataTmp := strings.Split(line, " ")
		lineData := []string{}
		for _, d := range lineDataTmp {
			if strings.Trim(d, " ") == "" {
				continue
			}
			lineData = append(lineData, d)
		}

		if len(lineData) < 7 {
			continue
		}
		cmdlineArgs, err := getCmdLine(lineData[0])
		if err != nil {
			log.WithError(err).WithFields(log.Fields{
				"command": lineData[6],
				"pid":     lineData[0],
			}).Error("Error getting command line")
			continue
		}

		if len(cmdlineArgs) == 0 {
			// No cmdline file, may be that process died
			continue
		}

		var lbls prometheus.Labels
		switch lineData[6] {
		case "glusterd":
			lbls = getGlusterdLabels(peerID, lineData[6], cmdlineArgs)
		case "glusterd2":
			lbls = getGlusterdLabels(peerID, lineData[6], cmdlineArgs)
		case "glusterfsd":
			lbls = getGlusterFsdLabels(peerID, lineData[6], cmdlineArgs)
		default:
			lbls = getUnknownLabels(peerID, lineData[6], cmdlineArgs)
		}

		vals := MLabels(labels).ValuesInOrder(lbls)
		pcpu, err := strconv.ParseFloat(lineData[1], 64)
		if err != nil {
			log.WithError(err).WithFields(log.Fields{
				"value":   lineData[1],
				"command": lineData[6],
				"pid":     lineData[0],
			}).Error("Unable to parse pcpu value")
			continue
		}

		pmem, err := strconv.ParseFloat(lineData[2], 64)
		if err != nil {
			log.WithError(err).WithFields(log.Fields{
				"value":   lineData[2],
				"command": lineData[6],
				"pid":     lineData[0],
			}).Error("Unable to parse pmem value")
			continue
		}
		rsz, err := strconv.ParseFloat(lineData[3], 64)
		if err != nil {
			log.WithError(err).WithFields(log.Fields{
				"value":   lineData[3],
				"command": lineData[6],
				"pid":     lineData[0],
			}).Error("Unable to parse rsz value")
			continue
		}

		vsz, err := strconv.ParseFloat(lineData[4], 64)
		if err != nil {
			log.WithError(err).WithFields(log.Fields{
				"value":   lineData[4],
				"command": lineData[6],
				"pid":     lineData[0],
			}).Error("Unable to parse vsz value")
			continue
		}

		etimes, err := strconv.ParseFloat(lineData[5], 64)
		if err != nil {
			log.WithError(err).WithFields(log.Fields{
				"value":   lineData[5],
				"command": lineData[6],
				"pid":     lineData[0],
			}).Error("Unable to parse etimes value")
			continue
		}

		// Convert to bytes from kilo bytes
		vsz = vsz * 1024
		rsz = rsz * 1024

		// Update the Metrics
		metricArr = append(metricArr, prometheus.MustNewConstMetric(
			glusterCPUPercentage,
			prometheus.GaugeValue,
			pcpu,
			vals...,
		))
		metricArr = append(metricArr, prometheus.MustNewConstMetric(
			glusterMemoryPercentage,
			prometheus.GaugeValue,
			pmem,
			vals...,
		))
		metricArr = append(metricArr, prometheus.MustNewConstMetric(
			glusterResidentMemory,
			prometheus.GaugeValue,
			rsz,
			vals...,
		))
		metricArr = append(metricArr, prometheus.MustNewConstMetric(
			glusterVirtualMemory,
			prometheus.GaugeValue,
			vsz,
			vals...,
		))
		metricArr = append(metricArr, prometheus.MustNewConstMetric(
			glusterElapsedTime,
			prometheus.GaugeValue,
			etimes,
			vals...,
		))
	}
	return metricArr, nil
}

func init() {
	gmPs := newGMetricGenProvider("gluster_ps", ps, psDescs)
	registerNewGMetricGenProvider(gmPs)
}
