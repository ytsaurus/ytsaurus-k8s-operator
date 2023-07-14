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
	ExcludeCategories []string      `yson:"exclude_categories,omitempty"`
	IncludeCategories []string      `yson:"include_categories,omitempty"`
	MinLevel          ytv1.LogLevel `yson:"min_level,omitempty"`
	Writers           []string      `yson:"writers,omitempty"`
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

func createLoggingRule(spec ytv1.LoggerSpec) LoggingRule {
	loggingRule := LoggingRule{
		MinLevel: spec.MinLogLevel,
		Writers:  []string{spec.Name},
	}

	if spec.CategoriesFilter != nil {
		switch spec.CategoriesFilter.Type {
		case ytv1.CategoriesFilterTypeExclude:
			loggingRule.ExcludeCategories = append(loggingRule.ExcludeCategories, spec.CategoriesFilter.Values...)

		case ytv1.CategoriesFilterTypeInclude:
			loggingRule.IncludeCategories = append(loggingRule.IncludeCategories, spec.CategoriesFilter.Values...)
		}
	}
	return loggingRule
}

func createLoggingWriter(componentName string, loggingDirectory string, loggerSpec ytv1.LoggerSpec) LoggingWriter {
	loggingWriter := LoggingWriter{
		WriterType: loggerSpec.WriterType,
	}

	if loggingWriter.WriterType == ytv1.LogWriterTypeFile {
		loggingWriter.FileName = path.Join(loggingDirectory, fmt.Sprintf("%s.%s.log", componentName, loggerSpec.Name))
	}

	return loggingWriter
}

func (b *loggingBuilder) addLogger(loggerSpec ytv1.LoggerSpec) *loggingBuilder {
	b.logging.Rules = append(b.logging.Rules, createLoggingRule(loggerSpec))
	b.logging.Writers[loggerSpec.Name] = createLoggingWriter(b.componentName, b.loggingDirectory, loggerSpec)

	return b
}
