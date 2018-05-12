package supervisor

import (
	"encoding/json"
	"github.com/shirou/gopsutil/cpu"
	"github.com/shirou/gopsutil/disk"
	"github.com/shirou/gopsutil/load"
	"github.com/shirou/gopsutil/mem"
	"github.com/shirou/gopsutil/net"
)

func MarshalMetrics() ([]byte, error) {
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
	return json.Marshal(metricsMap)
}
