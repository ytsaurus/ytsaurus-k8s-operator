package ytconfig

import (
	"fmt"
	"path"

	ytv1 "github.com/ytsaurus/yt-k8s-operator/api/v1"
)

func defaultStderrLoggerSpec() ytv1.LoggerSpec {
	return ytv1.LoggerSpec{
		Name:        "stderr",
		MinLogLevel: ytv1.LogLevelError,
		WriterType:  ytv1.LogWriterTypeStderr,
	}
}

func defaultDebugLoggerSpec() ytv1.LoggerSpec {
	return ytv1.LoggerSpec{
		Name:        "debug",
		MinLogLevel: ytv1.LogLevelDebug,
		WriterType:  ytv1.LogWriterTypeFile,
	}
}

func defaultInfoLoggerSpec() ytv1.LoggerSpec {
	return ytv1.LoggerSpec{
		Name:        "info",
		MinLogLevel: ytv1.LogLevelInfo,
		WriterType:  ytv1.LogWriterTypeFile,
	}
}

type LoggingRule struct {
	MinLevel ytv1.LogLevel `yson:"min_level,omitempty"`
	Writers  []string      `yson:"writers,omitempty"`
}

type LoggingWriter struct {
	WriterType ytv1.LogWriterType `yson:"type,omitempty"`
	FileName   string             `yson:"file_name,omitempty"`
}

type Logging struct {
	Writers map[string]LoggingWriter `yson:"writers"`
	Rules   []LoggingRule            `yson:"rules"`
}

type loggingBuilder struct {
	loggingDirectory string
	componentName    string
	logging          Logging
}

func newLoggingBuilder(location *ytv1.LocationSpec, componentName string) loggingBuilder {
	loggingDirectory := "/var/log"
	if location != nil {
		loggingDirectory = location.Path
	}

	return loggingBuilder{
		loggingDirectory: loggingDirectory,
		componentName:    componentName,
		logging: Logging{
			Rules:   make([]LoggingRule, 0),
			Writers: make(map[string]LoggingWriter),
		},
	}
}

func (b *loggingBuilder) addLogger(loggerSpec ytv1.LoggerSpec) *loggingBuilder {
	b.logging.Rules = append(b.logging.Rules, LoggingRule{
		MinLevel: loggerSpec.MinLogLevel,
		Writers:  []string{loggerSpec.Name},
	})

	writer := LoggingWriter{
		WriterType: loggerSpec.WriterType,
	}

	if writer.WriterType == ytv1.LogWriterTypeFile {
		writer.FileName = path.Join(b.loggingDirectory, fmt.Sprintf("%s.%s.log", b.componentName, loggerSpec.Name))
	}

	b.logging.Writers[loggerSpec.Name] = writer

	return b
}
