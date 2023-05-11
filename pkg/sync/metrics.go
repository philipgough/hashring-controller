package sync

import (
	"crypto/md5"
	"encoding/binary"

	"github.com/prometheus/client_golang/prometheus"
)

type metrics struct {
	configMapLastWriteSuccessTime prometheus.Gauge
	configMapHash                 prometheus.Gauge
}

func newMetrics() *metrics {
	return &metrics{
		configMapLastWriteSuccessTime: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "hashring_sync_controller_last_write_success_timestamp_seconds",
			Help: "Unix timestamp of the last successful write to disk",
		}),
		configMapHash: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "hashring__sync_controller_configmap_hash",
			Help: "Hash of the last created or updated configmap.",
		}),
	}
}

// hashAsMetricValue generates metric value from hash of data.
func hashAsMetricValue(data []byte) float64 {
	sum := md5.Sum(data)
	// We only want 48 bits as a float64 only has a 53 bit mantissa.
	smallSum := sum[0:6]
	bytes := make([]byte, 8)
	copy(bytes, smallSum)

	return float64(binary.LittleEndian.Uint64(bytes))
}
