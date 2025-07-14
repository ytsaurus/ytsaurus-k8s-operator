package ytconfig

import (
	"fmt"
	"path"

	ytv1 "github.com/ytsaurus/ytsaurus-k8s-operator/api/v1"
)

type TimbertruckConfig struct {
	WorkDir  string                     `json:"work_dir"`
	JsonLogs []TimbertruckJsonLogConfig `json:"json_logs"`
}

type TimbertruckJsonLogConfig struct {
	Name    string                     `json:"name"`
	LogFile string                     `json:"log_file"`
	YTQueue []TimbertruckYTQueueConfig `json:"yt_queue"`
}

type TimbertruckYTQueueConfig struct {
	Cluster      string `json:"cluster"`
	QueuePath    string `json:"queue_path"`
	ProducerPath string `json:"producer_path"`
}

func NewTimbertruckConfig(structuredLoggers []ytv1.StructuredLoggerSpec, workDir, componentName, loggingDirectory, deliveryProxy, logsDeliveryPath string) *TimbertruckConfig {
	timbertruckConfig := &TimbertruckConfig{
		WorkDir:  workDir,
		JsonLogs: []TimbertruckJsonLogConfig{},
	}

	for _, structuredLogger := range structuredLoggers {
		deliveryName := fmt.Sprintf("%s-%s", componentName, structuredLogger.Name)

		fileName := path.Join(loggingDirectory, fmt.Sprintf("%s.%s.log", componentName, structuredLogger.Name))
		if structuredLogger.Format != ytv1.LogFormatPlainText {
			fileName += fmt.Sprintf(".%s", structuredLogger.Format)
		}
		if structuredLogger.Compression != ytv1.LogCompressionNone {
			fileName += fmt.Sprintf(".%s", structuredLogger.Compression)
		}

		timbertruckJsonLogConfig := TimbertruckJsonLogConfig{
			Name:    deliveryName,
			LogFile: fileName,
			YTQueue: []TimbertruckYTQueueConfig{},
		}

		deliveryPath := fmt.Sprintf("%s/%s", logsDeliveryPath, deliveryName)

		timbertruckJsonLogConfig.YTQueue = append(timbertruckJsonLogConfig.YTQueue, TimbertruckYTQueueConfig{
			Cluster:      deliveryProxy,
			QueuePath:    fmt.Sprintf("%s/queue", deliveryPath),
			ProducerPath: fmt.Sprintf("%s/producer", deliveryPath),
		})

		timbertruckConfig.JsonLogs = append(timbertruckConfig.JsonLogs, timbertruckJsonLogConfig)
	}

	if len(timbertruckConfig.JsonLogs) == 0 {
		return nil
	}

	return timbertruckConfig
}
