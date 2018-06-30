package supervisor

import (
	"encoding/json"
	"github.com/openinx/huker/pkg/utils"
	"github.com/qiniu/log"
	"github.com/shirou/gopsutil/cpu"
	"github.com/shirou/gopsutil/disk"
	"github.com/shirou/gopsutil/load"
	"github.com/shirou/gopsutil/mem"
	"github.com/shirou/gopsutil/net"
	"strings"
)

func progsJVMMetrics(progs *programMap) map[string]interface{} {
	pMetrics := make(map[string]interface{})
	for pKey, prog := range progs.programs {
		if strings.Contains(prog.Bin, "java") && utils.IsProcessOK(prog.PID) {
			javaHome, err := utils.FindJavaHome(prog.Bin)
			if err != nil {
				log.Warnf("Failed to find the JAVA_HOME for %s, error: %v", pKey, err)
				continue
			}
			if mp, err := JstatGC(javaHome, prog.PID); err != nil {
				log.Warnf("Failed to collect `jstat -gc %d` for %s, error: %v", prog.PID, pKey, err)
				continue
			} else {
				pMetrics[pKey] = mp
			}
		}
	}
	return pMetrics
}

func MarshalMetrics(progs *programMap) ([]byte, error) {
	metricsMap := make(map[string]interface{})

	// Memory Statistic
	if memStat, err := mem.VirtualMemory(); err != nil {
		return nil, err
	} else {
		metricsMap["memory"] = memStat
	}

	// CPU Usage Percent
	if cpuPercents, err := cpu.Percent(-1, false); err != nil {
		return nil, err
	} else {
		metricsMap["CpuPercents"] = cpuPercents[0]
	}

	// CPU Time Stat
	if cpuTimeStat, err := cpu.Times(false); err != nil {
		return nil, err
	} else {
		metricsMap["CpuTimeStat"] = cpuTimeStat[0]
	}

	// Load1 / Load5 / Load10
	if avgLoad, err := load.Avg(); err != nil {
		return nil, err
	} else {
		metricsMap["Load"] = avgLoad
	}

	// Network
	if netStat, err := net.IOCounters(true); err != nil {
		return nil, err
	} else {
		metricsMap["Network"] = netStat
	}

	// Disk IO Stat
	if diskStat, err := disk.IOCounters(); err != nil {
		return nil, err
	} else {
		metricsMap["DiskIO"] = diskStat
	}

	// Disk Usage Stat
	if partitions, err := disk.Partitions(false); err != nil {
		return nil, err
	} else {
		deviceMap := make(map[string]interface{})
		for _, part := range partitions {
			deviceMap[part.Device] = part
			if usage, err := disk.Usage(part.Mountpoint); err != nil {
				return nil, err
			} else {
				deviceMap["usage:"+part.Device] = usage
			}
		}
		metricsMap["DiskUsage"] = deviceMap
	}

	// Program's metrics
	metricsMap["JavaMetrics"] = progsJVMMetrics(progs)

	return json.Marshal(metricsMap)
}
