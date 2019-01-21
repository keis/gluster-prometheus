package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"

	"github.com/gluster/gluster-prometheus/pkg/glusterutils"

	"github.com/prometheus/client_golang/prometheus"
	log "github.com/sirupsen/logrus"
)

var (
	brickLabels = []MetricLabel{
		{
			Name: "host",
			Help: "Host name or IP",
		},
		{
			Name: "id",
			Help: "Brick ID",
		},
		{
			Name: "brick_path",
			Help: "Brick Path",
		},
		{
			Name: "volume",
			Help: "Volume Name",
		},
		{
			Name: "subvolume",
			Help: "Sub Volume name",
		},
	}

	subvolLabels = []MetricLabel{
		{
			Name: "volume",
			Help: "Volume Name",
		},
		{
			Name: "subvolume",
			Help: "Sub volume name",
		},
	}

	lvmLbls = []MetricLabel{
		{
			Name: "host",
			Help: "Host name or IP",
		},
		{
			Name: "id",
			Help: "Brick ID",
		},
		{
			Name: "brick_path",
			Help: "Brick Path",
		},
		{
			Name: "volume",
			Help: "Volume Name",
		},
		{
			Name: "subvolume",
			Help: "Sub Volume name",
		},
		{
			Name: "vg_name",
			Help: "VG Name",
		},
		{
			Name: "lv_path",
			Help: "LV Path",
		},
		{
			Name: "lv_uuid",
			Help: "UUID of LV",
		},
	}

	brickStatusLbls = []MetricLabel{
		{
			Name: "volume",
			Help: "Volume Name",
		},
		{
			Name: "hostname",
			Help: "Host name or IP",
		},
		{
			Name: "brick_path",
			Help: "Brick Path",
		},
		{
			Name: "peer_id",
			Help: "Peer ID",
		},
		{
			Name: "pid",
			Help: "Process ID of brick",
		},
	}

	thinLvmLbls = []MetricLabel{
		{
			Name: "host",
			Help: "Host name or IP",
		},
		{
			Name: "thinpool_name",
			Help: "Name of the thinpool LV",
		},
		{
			Name: "vg_name",
			Help: "Name of the Volume Group",
		},
		{
			Name: "volume",
			Help: "Volume Name",
		},
		{
			Name: "subvolume",
			Help: "Name of the Subvolume",
		},
		{
			Name: "brick_path",
			Help: "Brick Path",
		},
	}

	// brickDescs collects all the brick metric 'Desc'-s
	brickDescs []*prometheus.Desc
	// brickStatusDescs collects all the brick status metric 'Desc'-s
	brickStatusDescs []*prometheus.Desc

	glusterBrickCapacityUsed = newPrometheusDesc(Metric{
		Namespace: "gluster",
		Name:      "brick_capacity_used_bytes",
		Help:      "Used capacity of gluster bricks in bytes",
		LongHelp:  "",
		Labels:    brickLabels,
	}, &brickDescs)

	glusterBrickCapacityFree = newPrometheusDesc(Metric{
		Namespace: "gluster",
		Name:      "brick_capacity_free_bytes",
		Help:      "Free capacity of gluster bricks in bytes",
		LongHelp:  "",
		Labels:    brickLabels,
	}, &brickDescs)

	glusterBrickCapacityTotal = newPrometheusDesc(Metric{
		Namespace: "gluster",
		Name:      "brick_capacity_bytes_total",
		Help:      "Total capacity of gluster bricks in bytes",
		LongHelp:  "",
		Labels:    brickLabels,
	}, &brickDescs)

	glusterBrickInodesTotal = newPrometheusDesc(Metric{
		Namespace: "gluster",
		Name:      "brick_inodes_total",
		Help:      "Total no of inodes of gluster brick disk",
		LongHelp:  "",
		Labels:    brickLabels,
	}, &brickDescs)

	glusterBrickInodesFree = newPrometheusDesc(Metric{
		Namespace: "gluster",
		Name:      "brick_inodes_free",
		Help:      "Free no of inodes of gluster brick disk",
		LongHelp:  "",
		Labels:    brickLabels,
	}, &brickDescs)

	glusterBrickInodesUsed = newPrometheusDesc(Metric{
		Namespace: "gluster",
		Name:      "brick_inodes_used",
		Help:      "Used no of inodes of gluster brick disk",
		LongHelp:  "",
		Labels:    brickLabels,
	}, &brickDescs)

	glusterSubvolCapacityUsed = newPrometheusDesc(Metric{
		Namespace: "gluster",
		Name:      "subvol_capacity_used_bytes",
		Help:      "Effective used capacity of gluster subvolume in bytes",
		LongHelp:  "",
		Labels:    subvolLabels,
	}, &brickDescs)

	glusterSubvolCapacityTotal = newPrometheusDesc(Metric{
		Namespace: "gluster",
		Name:      "subvol_capacity_total_bytes",
		Help:      "Effective total capacity of gluster subvolume in bytes",
		LongHelp:  "",
		Labels:    subvolLabels,
	}, &brickDescs)

	glusterBrickLVSize = newPrometheusDesc(Metric{
		Namespace: "gluster",
		Name:      "brick_lv_size_bytes",
		Help:      "Bricks LV size Bytes",
		LongHelp:  "",
		Labels:    lvmLbls,
	}, &brickDescs)

	glusterBrickLVPercent = newPrometheusDesc(Metric{
		Namespace: "gluster",
		Name:      "brick_lv_percent",
		Help:      "Bricks LV usage percent",
		LongHelp:  "",
		Labels:    lvmLbls,
	}, &brickDescs)

	glusterBrickLVMetadataSize = newPrometheusDesc(Metric{
		Namespace: "gluster",
		Name:      "brick_lv_metadata_size_bytes",
		Help:      "Bricks LV metadata size Bytes",
		LongHelp:  "",
		Labels:    lvmLbls,
	}, &brickDescs)

	glusterBrickLVMetadataPercent = newPrometheusDesc(Metric{
		Namespace: "gluster",
		Name:      "brick_lv_metadata_percent",
		Help:      "Bricks LV metadata usage percent",
		LongHelp:  "",
		Labels:    lvmLbls,
	}, &brickDescs)

	glusterVGExtentTotal = newPrometheusDesc(Metric{
		Namespace: "gluster",
		Name:      "vg_extent_total_count",
		Help:      "VG extent total count ",
		LongHelp:  "",
		Labels:    lvmLbls,
	}, &brickDescs)

	glusterVGExtentAlloc = newPrometheusDesc(Metric{
		Namespace: "gluster",
		Name:      "vg_extent_alloc_count",
		Help:      "VG extent allocated count ",
		LongHelp:  "",
		Labels:    lvmLbls,
	}, &brickDescs)

	glusterThinPoolDataTotal = newPrometheusDesc(Metric{
		Namespace: "gluster",
		Name:      "thinpool_data_total_bytes",
		Help:      "Thin pool size Bytes",
		LongHelp:  "",
		Labels:    thinLvmLbls,
	}, &brickDescs)

	glusterThinPoolDataUsed = newPrometheusDesc(Metric{
		Namespace: "gluster",
		Name:      "thinpool_data_used_bytes",
		Help:      "Thin pool data used Bytes",
		LongHelp:  "",
		Labels:    thinLvmLbls,
	}, &brickDescs)

	glusterThinPoolMetadataTotal = newPrometheusDesc(Metric{
		Namespace: "gluster",
		Name:      "thinpool_metadata_total_bytes",
		Help:      "Thin pool metadata size Bytes",
		LongHelp:  "",
		Labels:    thinLvmLbls,
	}, &brickDescs)

	glusterThinPoolMetadataUsed = newPrometheusDesc(Metric{
		Namespace: "gluster",
		Name:      "thinpool_metadata_used_bytes",
		Help:      "Thin pool metadata used Bytes",
		LongHelp:  "",
		Labels:    thinLvmLbls,
	}, &brickDescs)

	glusterBrickUp = newPrometheusDesc(Metric{
		Namespace: "gluster",
		Name:      "brick_up",
		Help:      "Brick up (1-up, 0-down)",
		LongHelp:  "",
		Labels:    brickStatusLbls,
	}, &brickStatusDescs)
)

func getGlusterBrickLabels(brick glusterutils.Brick, subvol string) prometheus.Labels {
	return prometheus.Labels{
		"host":       brick.Host,
		"id":         brick.ID,
		"brick_path": brick.Path,
		"volume":     brick.VolumeName,
		"subvolume":  subvol,
	}
}

func getGlusterSubvolLabels(volname string, subvol string) prometheus.Labels {
	return prometheus.Labels{
		"volume":    volname,
		"subvolume": subvol,
	}
}

// DiskStatus represents Disk usage
type DiskStatus struct {
	All        float64 `json:"all"`
	Used       float64 `json:"used"`
	Free       float64 `json:"free"`
	InodesAll  float64 `json:"inodesall"`
	InodesFree float64 `json:"inodesfree"`
	InodesUsed float64 `json:"inodesused"`
}

func diskUsage(path string) (disk DiskStatus, err error) {
	fs := syscall.Statfs_t{}
	err = syscall.Statfs(path, &fs)
	if err != nil {
		return
	}
	disk.All = float64(fs.Blocks * uint64(fs.Bsize))
	disk.Free = float64(fs.Bfree * uint64(fs.Bsize))
	disk.Used = disk.All - disk.Free
	disk.InodesAll = float64(fs.Files)
	disk.InodesFree = float64(fs.Ffree)
	disk.InodesUsed = disk.InodesAll - disk.InodesFree
	return
}

// LVMStat represents LVM details
type LVMStat struct {
	Device          string
	UUID            string
	Name            string
	DataPercent     float64
	PoolLV          string
	Attr            string
	Size            float64
	Path            string
	MetadataSize    float64
	MetadataPercent float64
	VGName          string
	VGExtentTotal   float64
	VGExtentAlloc   float64
}

// ThinPoolStat represents thin pool LV details
type ThinPoolStat struct {
	ThinPoolName          string
	ThinPoolVGName        string
	ThinPoolDataTotal     float64
	ThinPoolDataUsed      float64
	ThinPoolMetadataTotal float64
	ThinPoolMetadataUsed  float64
}

// VGReport represents VG details
type VGReport struct {
	Report []VGs `json:"report"`
}

// VGs represents list VG Details
type VGs struct {
	Vgs []VGDetails `json:"vg"`
}

// VGDetails represents a single VG detail
type VGDetails struct {
	LVUUID          string `json:"lv_uuid"`
	LVName          string `json:"lv_name"`
	DataPercent     string `json:"data_percent"`
	PoolLV          string `json:"pool_lv"`
	LVAttr          string `json:"lv_attr"`
	LVSize          string `json:"lv_size"`
	LVPath          string `json:"lv_path"`
	LVMetadataSize  string `json:"lv_metadata_size"`
	MetadataPercent string `json:"metadata_percent"`
	VGName          string `json:"vg_name"`
	VGExtentTotal   string `json:"vg_extent_count"`
	VGExtentFree    string `json:"vg_free_count"`
}

func getLVS() ([]LVMStat, []ThinPoolStat, error) {
	cmd := "lvm vgs --unquoted --reportformat=json --noheading --nosuffix --units m -o lv_uuid,lv_name,data_percent,pool_lv,lv_attr,lv_size,lv_path,lv_metadata_size,metadata_percent,vg_name,vg_extent_count,vg_free_count"

	out, err := exec.Command("sh", "-c", cmd).Output()
	lvmDet := []LVMStat{}
	thinPool := []ThinPoolStat{}
	var vgExtentFreeTemp float64
	if err != nil {
		log.WithError(err).Error("Error getting lvm usage details")
		return lvmDet, thinPool, err
	}
	var vgReport VGReport
	if err1 := json.Unmarshal(out, &vgReport); err1 != nil {
		log.WithError(err1).Error("Error parsing lvm usage details")
		return lvmDet, thinPool, err1
	}

	for _, vg := range vgReport.Report[0].Vgs {
		var obj LVMStat
		obj.UUID = vg.LVUUID
		obj.Name = vg.LVName
		if vg.DataPercent == "" {
			obj.DataPercent = 0.0
		} else {
			if obj.DataPercent, err = strconv.ParseFloat(vg.DataPercent, 64); err != nil {
				log.WithError(err).Error("Error parsing DataPercent value of lvm usage")
				return lvmDet, thinPool, err
			}
		}
		obj.PoolLV = vg.PoolLV
		obj.Attr = vg.LVAttr
		if vg.LVSize == "" {
			obj.Size = 0.0
		} else {
			if obj.Size, err = strconv.ParseFloat(vg.LVSize, 64); err != nil {
				log.WithError(err).Error("Error parsing LVSize value of lvm usage")
				return lvmDet, thinPool, err
			}
		}
		obj.Path = vg.LVPath
		if vg.LVMetadataSize == "" {
			obj.MetadataSize = 0.0
		} else {
			if obj.MetadataSize, err = strconv.ParseFloat(vg.LVMetadataSize, 64); err != nil {
				log.WithError(err).Error("Error parsing LVMetadataSize value of lvm usage")
				return lvmDet, thinPool, err
			}
		}
		if vg.MetadataPercent == "" {
			obj.MetadataPercent = 0.0
		} else {
			obj.MetadataPercent, err = strconv.ParseFloat(vg.MetadataPercent, 64)
			if err != nil {
				log.WithError(err).Error("Error parsing MetadataPercent value of lvm usage")
				return lvmDet, thinPool, err
			}
		}
		if vg.VGExtentTotal == "" {
			obj.VGExtentTotal = 0.0
		} else {
			obj.VGExtentTotal, err = strconv.ParseFloat(vg.VGExtentTotal, 64)
			if err != nil {
				log.WithError(err).Error("Error parsing VGExtenTotal value of lvm usage")
				return lvmDet, thinPool, err
			}
		}
		if vg.VGExtentFree == "" {
			vgExtentFreeTemp = 0.0
		} else {
			vgExtentFreeTemp, err = strconv.ParseFloat(vg.VGExtentFree, 64)
			if err != nil {
				log.WithError(err).Error("Error parsing VGExtentAlloc value of lvm usage")
				return lvmDet, thinPool, err
			}
		}
		obj.VGExtentAlloc = obj.VGExtentTotal - vgExtentFreeTemp
		obj.VGName = vg.VGName
		if obj.Attr[0] == 't' {
			obj.Device = fmt.Sprintf("%s/%s", obj.VGName, obj.Name)
			var TPUsage ThinPoolStat
			TPUsage.ThinPoolName = obj.Name
			TPUsage.ThinPoolVGName = obj.VGName
			TPUsage.ThinPoolDataTotal = obj.Size
			TPUsage.ThinPoolDataUsed = (obj.Size * obj.DataPercent) / 100
			TPUsage.ThinPoolMetadataTotal = obj.MetadataSize
			TPUsage.ThinPoolMetadataUsed = (obj.MetadataSize * obj.MetadataPercent) / 100
			thinPool = append(thinPool, TPUsage)
		} else {
			obj.Device, err = filepath.EvalSymlinks(obj.Path)
			if err != nil {
				log.WithError(err).WithFields(log.Fields{
					"path": obj.Path,
				}).Error("Error evaluating realpath")
				return lvmDet, thinPool, err
			}
		}
		lvmDet = append(lvmDet, obj)
	}
	return lvmDet, thinPool, nil
}

// ProcMounts represents list of items from /proc/mounts
type ProcMounts struct {
	Name         string
	Device       string
	FSType       string
	MountOptions string
}

func parseProcMounts() ([]ProcMounts, error) {
	procMounts := []ProcMounts{}
	b, err := ioutil.ReadFile("/proc/mounts")
	if err != nil {
		return procMounts, err
	}
	for _, line := range strings.Split(string(b), "\n") {
		if strings.HasPrefix(line, "/") {
			tokens := strings.Fields(line)
			procMounts = append(procMounts,
				ProcMounts{Name: tokens[1], Device: tokens[0], FSType: tokens[2], MountOptions: tokens[3]})
		}
	}
	return procMounts, nil
}

func getGlusterLVMLabels(brick glusterutils.Brick,
	subvol string, stat LVMStat) prometheus.Labels {
	return prometheus.Labels{
		"host":       brick.Host,
		"id":         brick.ID,
		"brick_path": brick.Path,
		"volume":     brick.VolumeName,
		"subvolume":  subvol,
		"vg_name":    stat.VGName,
		"lv_path":    stat.Path,
		"lv_uuid":    stat.UUID,
	}
}

func getGlusterThinPoolLabels(brick glusterutils.Brick, vol string,
	subvol string, thinStat ThinPoolStat) prometheus.Labels {
	return prometheus.Labels{
		"host":          brick.Host,
		"thinpool_name": thinStat.ThinPoolName,
		"vg_name":       thinStat.ThinPoolVGName,
		"volume":        vol,
		"subvolume":     subvol,
		"brick_path":    brick.Path,
	}
}

func lvmUsage(path string) (stats []LVMStat, thinPoolStats []ThinPoolStat, err error) {
	mountPoints, err := parseProcMounts()
	if err != nil {
		return stats, thinPoolStats, err
	}
	var thinPoolNames []string
	lvs, tpStats, err := getLVS()
	if err != nil {
		return stats, thinPoolStats, err
	}
	for _, lv := range lvs {
		for _, mount := range mountPoints {
			dev, err := filepath.EvalSymlinks(mount.Device)
			if err != nil {
				log.WithError(err).WithFields(log.Fields{
					"path": mount.Device,
				}).Error("Error evaluating realpath")
				continue
			}
			// Check if the logical volume is mounted as a gluster brick
			if lv.Device == dev && strings.HasPrefix(path, mount.Name) {
				// Check if the LV is a thinly provisioned volume and if yes then get the thin pool LV name
				if lv.Attr[0] == 'V' {
					tpName := lv.PoolLV
					thinPoolNames = append(thinPoolNames, tpName)
				}
				stats = append(stats, lv)
			}
		}
	}
	// Iterate and select only those thin pool LVs whose thinly provisioned volumes are mounted as gluster bricks
	for _, tpName := range thinPoolNames {
		for _, tpStat := range tpStats {
			if tpName == tpStat.ThinPoolName {
				thinPoolStats = append(thinPoolStats, tpStat)
			}
		}
	}

	return stats, thinPoolStats, nil
}

func brickUtilization(gluster glusterutils.GInterface) ([]prometheus.Metric, error) {
	volumes, err := gluster.VolumeInfo()
	if err != nil {
		// Return without exporting metric in this cycle
		return nil, err
	}

	localPeerID, err := gluster.LocalPeerID()
	if err != nil {
		// Return without exporting metric in this cycle
		return nil, err
	}

	var metricArr []prometheus.Metric
	for _, volume := range volumes {
		if volume.State != glusterutils.VolumeStateStarted {
			// Export brick metrics only if the Volume
			// is is in Started state
			continue
		}
		subvols := volume.SubVolumes
		for _, subvol := range subvols {
			bricks := subvol.Bricks
			var maxBrickUsed float64
			var leastBrickTotal float64
			for _, brick := range bricks {
				if brick.PeerID == localPeerID {
					usage, err := diskUsage(brick.Path)
					if err != nil {
						log.WithError(err).WithFields(log.Fields{
							"volume":     volume.Name,
							"brick_path": brick.Path,
						}).Error("Error getting disk usage")
						continue
					}
					var lbls = getGlusterBrickLabels(brick, subvol.Name)
					var vals = MLabels(brickLabels).ValuesInOrder(lbls)
					// Update the metrics
					metricArr = append(metricArr, prometheus.MustNewConstMetric(
						glusterBrickCapacityUsed,
						prometheus.GaugeValue,
						usage.Used,
						vals...,
					))
					metricArr = append(metricArr, prometheus.MustNewConstMetric(
						glusterBrickCapacityFree,
						prometheus.GaugeValue,
						usage.Free,
						vals...,
					))
					metricArr = append(metricArr, prometheus.MustNewConstMetric(
						glusterBrickCapacityTotal,
						prometheus.GaugeValue,
						usage.All,
						vals...,
					))
					metricArr = append(metricArr, prometheus.MustNewConstMetric(
						glusterBrickInodesTotal,
						prometheus.GaugeValue,
						usage.InodesAll,
						vals...,
					))
					metricArr = append(metricArr, prometheus.MustNewConstMetric(
						glusterBrickInodesFree,
						prometheus.GaugeValue,
						usage.InodesFree,
						vals...,
					))
					metricArr = append(metricArr, prometheus.MustNewConstMetric(
						glusterBrickInodesUsed,
						prometheus.GaugeValue,
						usage.InodesUsed,
						vals...,
					))
					// Skip exporting utilization data in case of arbiter
					// brick to avoid wrong values when both the data bricks
					// are down
					if brick.Type != glusterutils.BrickTypeArbiter && usage.Used >= maxBrickUsed {
						maxBrickUsed = usage.Used
					}
					if brick.Type != glusterutils.BrickTypeArbiter {
						if leastBrickTotal == 0 || usage.All <= leastBrickTotal {
							leastBrickTotal = usage.All
						}
					}
					// Get lvm usage details
					stats, thinStats, err := lvmUsage(brick.Path)
					if err != nil {
						log.WithError(err).WithFields(log.Fields{
							"volume":     volume.Name,
							"brick_path": brick.Path,
						}).Error("Error getting lvm usage")
						continue
					}
					// Add metrics
					for _, stat := range stats {
						var lvmLabels = getGlusterLVMLabels(brick, subvol.Name, stat)
						var valsLbls = MLabels(lvmLbls).ValuesInOrder(lvmLabels)
						// Convert to bytes
						metricArr = append(metricArr, prometheus.MustNewConstMetric(
							glusterBrickLVSize,
							prometheus.GaugeValue,
							stat.Size*1024*1024,
							valsLbls...,
						))
						metricArr = append(metricArr, prometheus.MustNewConstMetric(
							glusterBrickLVPercent,
							prometheus.GaugeValue,
							stat.DataPercent,
							valsLbls...,
						))
						metricArr = append(metricArr, prometheus.MustNewConstMetric(
							glusterBrickLVMetadataSize,
							prometheus.GaugeValue,
							// Convert to bytes
							stat.MetadataSize*1024*1024,
							valsLbls...,
						))
						metricArr = append(metricArr, prometheus.MustNewConstMetric(
							glusterBrickLVMetadataPercent,
							prometheus.GaugeValue,
							stat.MetadataPercent,
							valsLbls...,
						))
						metricArr = append(metricArr, prometheus.MustNewConstMetric(
							glusterVGExtentTotal,
							prometheus.GaugeValue,
							stat.VGExtentTotal,
							valsLbls...,
						))
						metricArr = append(metricArr, prometheus.MustNewConstMetric(
							glusterVGExtentAlloc,
							prometheus.GaugeValue,
							stat.VGExtentAlloc,
							valsLbls...,
						))
					}
					for _, thinStat := range thinStats {
						var thinLvmLabels = getGlusterThinPoolLabels(brick, volume.Name, subvol.Name, thinStat)
						var thinLvmVals = MLabels(thinLvmLbls).ValuesInOrder(thinLvmLabels)
						metricArr = append(metricArr, prometheus.MustNewConstMetric(
							glusterThinPoolDataTotal,
							prometheus.GaugeValue,
							thinStat.ThinPoolDataTotal*1024*1024,
							thinLvmVals...,
						))
						metricArr = append(metricArr, prometheus.MustNewConstMetric(
							glusterThinPoolDataUsed,
							prometheus.GaugeValue,
							thinStat.ThinPoolDataUsed*1024*1024,
							thinLvmVals...,
						))
						metricArr = append(metricArr, prometheus.MustNewConstMetric(
							glusterThinPoolMetadataTotal,
							prometheus.GaugeValue,
							thinStat.ThinPoolMetadataTotal*1024*1024,
							thinLvmVals...,
						))
						metricArr = append(metricArr, prometheus.MustNewConstMetric(
							glusterThinPoolMetadataUsed,
							prometheus.GaugeValue,
							thinStat.ThinPoolMetadataUsed*1024*1024,
							thinLvmVals...,
						))
					}
				}
			}
			effectiveCapacity := maxBrickUsed
			effectiveTotalCapacity := leastBrickTotal
			var subvolLbls = getGlusterSubvolLabels(volume.Name, subvol.Name)
			var subvolLblsVals = MLabels(subvolLabels).ValuesInOrder(subvolLbls)
			if subvol.Type == glusterutils.SubvolTypeDisperse {
				// In disperse volume data bricks contribute to the sub
				// volume size
				effectiveCapacity = maxBrickUsed * float64(subvol.DisperseDataCount)
				effectiveTotalCapacity = leastBrickTotal * float64(subvol.DisperseDataCount)
			}

			// Export the metric only if available. it will be zero if the subvolume
			// contains only arbiter brick on current node or no local bricks on
			// this node
			if effectiveCapacity > 0 {
				metricArr = append(metricArr, prometheus.MustNewConstMetric(
					glusterSubvolCapacityUsed,
					prometheus.GaugeValue,
					effectiveCapacity,
					subvolLblsVals...,
				))
			}
			if effectiveTotalCapacity > 0 {
				metricArr = append(metricArr, prometheus.MustNewConstMetric(
					glusterSubvolCapacityTotal,
					prometheus.GaugeValue,
					effectiveTotalCapacity,
					subvolLblsVals...,
				))
			}
		}
	}
	return metricArr, nil
}

func getBrickStatusLabels(vol string, host string,
	brickPath string, peerID string, pid int) prometheus.Labels {
	return prometheus.Labels{
		"volume":     vol,
		"hostname":   host,
		"brick_path": brickPath,
		"peer_id":    peerID,
		"pid":        strconv.Itoa(pid),
	}
}

func brickStatus(gluster glusterutils.GInterface) ([]prometheus.Metric, error) {
	isLeader, err := gluster.IsLeader()
	if err != nil {
		log.WithError(err).Error("Unable to find if the current node is leader")
		return nil, err
	}
	if !isLeader {
		return nil, nil
	}

	volumes, err := gluster.VolumeInfo()
	if err != nil {
		return nil, err
	}
	var allMetrics []prometheus.Metric
	for _, volume := range volumes {
		// If volume is down, the bricks should be marked down
		var brickStatus []glusterutils.BrickStatus
		if volume.State != glusterutils.VolumeStateStarted {
			for _, subvol := range volume.SubVolumes {
				for _, brick := range subvol.Bricks {
					status := glusterutils.BrickStatus{
						Hostname: brick.Host,
						PeerID:   brick.PeerID,
						Status:   0,
						PID:      0,
						Path:     brick.Path,
						Volume:   volume.Name,
					}
					brickStatus = append(brickStatus, status)
				}
			}
		} else {
			brickStatus, err = gluster.VolumeBrickStatus(volume.Name)
			if err != nil {
				log.WithError(err).WithFields(log.Fields{
					"volume": volume.Name,
				}).Error("Error getting bricks status")
				continue
			}
		}
		for _, entry := range brickStatus {
			labels := getBrickStatusLabels(volume.Name, entry.Hostname, entry.Path, entry.PeerID, entry.PID)
			vals := MLabels(brickStatusLbls).ValuesInOrder(labels)
			allMetrics = append(allMetrics, prometheus.MustNewConstMetric(
				glusterBrickUp,
				prometheus.CounterValue,
				float64(entry.Status),
				vals...,
			))
		}
	}

	return allMetrics, nil
}

func init() {
	gBrickUtilMP := newGMetricGenProvider("gluster_brick", brickUtilization, brickDescs)
	registerNewGMetricGenProvider(gBrickUtilMP)
	gBrickStatusMP := newGMetricGenProvider("gluster_brick_status", brickStatus, brickStatusDescs)
	registerNewGMetricGenProvider(gBrickStatusMP)
}
