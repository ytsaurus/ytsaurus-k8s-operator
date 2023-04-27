package ytconfig

import (
	"fmt"
	ytv1 "github.com/ytsaurus/yt-k8s-operator/api/v1"
	"path"
)

type LogLevel string

const (
	LogLevelTrace LogLevel = "trace"
	LogLevelDebug LogLevel = "debug"
	LogLevelInfo  LogLevel = "info"
	LogLevelError LogLevel = "error"
)

type LogWriterType string

const (
	LogWriterTypeFile   LogWriterType = "file"
	LogWriterTypeStderr LogWriterType = "stderr"
)

type LoggingRule struct {
	MinLevel LogLevel `yson:"min_level,omitempty"`
	Writers  []string `yson:"writers,omitempty"`
}

type LoggingWriter struct {
	WriterType LogWriterType `yson:"type,omitempty"`
	FileName   string        `yson:"file_name,omitempty"`
}

type Logging struct {
	Writers map[string]LoggingWriter `yson:"writers"`
	Rules   []LoggingRule            `yson:"rules"`
}

type loggingBuilder struct {
	path          string
	componentName string
	logging       Logging
}

func newLoggingBuilder(location *ytv1.LocationSpec, componentName string) loggingBuilder {
	path := "/var/log"
	if location != nil {
		path = location.Path
	}

	return loggingBuilder{
		path:          path,
		componentName: componentName,
		logging: Logging{
			Rules:   make([]LoggingRule, 0),
			Writers: make(map[string]LoggingWriter),
		},
	}
}

func (b *loggingBuilder) addDefaultStderr() *loggingBuilder {
	writerName := "error"
	b.logging.Rules = append(b.logging.Rules, LoggingRule{
		MinLevel: LogLevelError,
		Writers:  []string{writerName},
	})

	b.logging.Writers[writerName] = LoggingWriter{
		WriterType: LogWriterTypeStderr,
	}

	return b
}

func (b *loggingBuilder) addDefaultDebug() *loggingBuilder {
	writerName := "debug"
	b.logging.Rules = append(b.logging.Rules, LoggingRule{
		MinLevel: LogLevelDebug,
		Writers:  []string{writerName},
	})

	b.logging.Writers[writerName] = LoggingWriter{
		WriterType: LogWriterTypeFile,
		FileName:   path.Join(b.path, fmt.Sprintf("%s.debug.log", b.componentName)),
	}

	return b
}

func (b *loggingBuilder) addDefaultInfo() *loggingBuilder {
	writerName := "info"
	b.logging.Rules = append(b.logging.Rules, LoggingRule{
		MinLevel: LogLevelInfo,
		Writers:  []string{writerName},
	})

	b.logging.Writers[writerName] = LoggingWriter{
		WriterType: LogWriterTypeFile,
		FileName:   path.Join(b.path, fmt.Sprintf("%s.log", b.componentName)),
	}

	return b
}
