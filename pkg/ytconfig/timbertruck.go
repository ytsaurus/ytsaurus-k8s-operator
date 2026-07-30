package ytconfig

import (
	"fmt"
	"path"

	"go.ytsaurus.tech/yt/go/yson"

	ytv1 "github.com/ytsaurus/ytsaurus-k8s-operator/api/v1"
)

type TimbertruckConfig struct {
	WorkDir  string                     `json:"work_dir" yson:"work_dir"`
	JsonLogs []TimbertruckJsonLogConfig `json:"json_logs" yson:"json_logs"`
}

const timbertruckQueueBatchSize = 8 * 1024 * 1024 // 8 MiB

type TimbertruckJsonLogConfig struct {
	Name           string                     `json:"name" yson:"name"`
	LogFile        string                     `json:"log_file" yson:"log_file"`
	QueueBatchSize int                        `json:"queue_batch_size" yson:"queue_batch_size"`
	YTQueue        []TimbertruckYTQueueConfig `json:"yt_queue" yson:"yt_queue"`
}

type TimbertruckYTQueueConfig struct {
	Cluster      string `json:"cluster" yson:"cluster"`
	QueuePath    string `json:"queue_path" yson:"queue_path"`
	ProducerPath string `json:"producer_path" yson:"producer_path"`
}

func NewTimbertruckConfig(
	structuredLoggers []ytv1.StructuredLoggerSpec,
	workDir,
	componentName,
	logsDirectory,
	deliveryProxy,
	logsDeliveryPath string,
) *TimbertruckConfig {
	timbertruckConfig := &TimbertruckConfig{
		WorkDir:  workDir,
		JsonLogs: []TimbertruckJsonLogConfig{},
	}

	for _, structuredLogger := range structuredLoggers {
		deliveryName := fmt.Sprintf("%s-%s", componentName, structuredLogger.Name)

		fileName := path.Join(logsDirectory, fmt.Sprintf("%s.%s.log", componentName, structuredLogger.Name))
		if structuredLogger.Format != ytv1.LogFormatPlainText {
			fileName += fmt.Sprintf(".%s", structuredLogger.Format)
		}
		if structuredLogger.Compression != ytv1.LogCompressionNone {
			fileName += fmt.Sprintf(".%s", structuredLogger.Compression)
		}

		timbertruckJsonLogConfig := TimbertruckJsonLogConfig{
			Name:           deliveryName,
			LogFile:        fileName,
			QueueBatchSize: timbertruckQueueBatchSize,
			YTQueue:        []TimbertruckYTQueueConfig{},
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

func (c *TimbertruckConfig) ToYSON() ([]byte, error) {
	return yson.MarshalFormat((*timbertruckConfigAlias)(c), yson.FormatPretty)
}

// timbertruckConfigAlias avoids recursive marshaling if TimbertruckConfig ever implements custom YSON marshaling (e.g. MarshalYSON).
type timbertruckConfigAlias TimbertruckConfig
