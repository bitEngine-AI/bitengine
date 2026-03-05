package monitor

import (
	"context"
	"fmt"

	"github.com/shirou/gopsutil/v4/cpu"
	"github.com/shirou/gopsutil/v4/disk"
	"github.com/shirou/gopsutil/v4/mem"
)

type SystemMetrics struct {
	CPU    CPUMetrics  `json:"cpu"`
	Memory MemMetrics  `json:"memory"`
	Disk   DiskMetrics `json:"disk"`
}

type CPUMetrics struct {
	UsagePercent float64 `json:"usage_percent"`
	Cores        int     `json:"cores"`
}

type MemMetrics struct {
	Total       uint64  `json:"total"`
	Used        uint64  `json:"used"`
	UsedPercent float64 `json:"used_percent"`
}

type DiskMetrics struct {
	Total       uint64  `json:"total"`
	Used        uint64  `json:"used"`
	UsedPercent float64 `json:"used_percent"`
}

func Collect(ctx context.Context) (*SystemMetrics, error) {
	cpuPercent, err := cpu.PercentWithContext(ctx, 0, false)
	if err != nil {
		return nil, fmt.Errorf("monitor: %w", err)
	}
	cores, err := cpu.CountsWithContext(ctx, true)
	if err != nil {
		return nil, fmt.Errorf("monitor: %w", err)
	}

	vmem, err := mem.VirtualMemoryWithContext(ctx)
	if err != nil {
		return nil, fmt.Errorf("monitor: %w", err)
	}

	diskUsage, err := disk.UsageWithContext(ctx, "/")
	if err != nil {
		return nil, fmt.Errorf("monitor: %w", err)
	}

	var cpuUsage float64
	if len(cpuPercent) > 0 {
		cpuUsage = cpuPercent[0]
	}

	return &SystemMetrics{
		CPU: CPUMetrics{
			UsagePercent: cpuUsage,
			Cores:        cores,
		},
		Memory: MemMetrics{
			Total:       vmem.Total,
			Used:        vmem.Used,
			UsedPercent: vmem.UsedPercent,
		},
		Disk: DiskMetrics{
			Total:       diskUsage.Total,
			Used:        diskUsage.Used,
			UsedPercent: diskUsage.UsedPercent,
		},
	}, nil
}
