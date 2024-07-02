package ytconfig

import (
	"fmt"
	"path"

	"k8s.io/utils/ptr"

	ytv1 "github.com/ytsaurus/yt-k8s-operator/api/v1"
)

func defaultStderrLoggerSpec() ytv1.TextLoggerSpec {
	return ytv1.TextLoggerSpec{
		BaseLoggerSpec: ytv1.BaseLoggerSpec{
			Name:               "stderr",
			MinLogLevel:        ytv1.LogLevelError,
			Compression:        ytv1.LogCompressionNone,
			UseTimestampSuffix: false,
			Format:             ytv1.LogFormatPlainText,
		},
		WriterType: ytv1.LogWriterTypeStderr,
	}
}

func defaultDebugLoggerSpec() ytv1.TextLoggerSpec {
	return ytv1.TextLoggerSpec{
		BaseLoggerSpec: ytv1.BaseLoggerSpec{
			Name:               "debug",
			MinLogLevel:        ytv1.LogLevelDebug,
			Compression:        ytv1.LogCompressionNone,
			UseTimestampSuffix: false,
			Format:             ytv1.LogFormatPlainText,
		},
		WriterType: ytv1.LogWriterTypeFile,
	}
}

func defaultInfoLoggerSpec() ytv1.TextLoggerSpec {
	return ytv1.TextLoggerSpec{
		BaseLoggerSpec: ytv1.BaseLoggerSpec{
			Name:               "info",
			MinLogLevel:        ytv1.LogLevelInfo,
			Compression:        ytv1.LogCompressionNone,
			UseTimestampSuffix: false,
			Format:             ytv1.LogFormatPlainText,
		},
		WriterType: ytv1.LogWriterTypeFile,
	}
}

type LogFamily string

const (
	LogFamilyPlainText  LogFamily = "plain_text"
	LogFamilyStructured LogFamily = "structured"
)

type LoggingRule struct {
	ExcludeCategories []string      `yson:"exclude_categories,omitempty"`
	IncludeCategories []string      `yson:"include_categories,omitempty"`
	MinLevel          ytv1.LogLevel `yson:"min_level,omitempty"`
	Writers           []string      `yson:"writers,omitempty"`
	Family            *LogFamily    `yson:"family,omitempty"`
}

type LoggingWriter struct {
	WriterType ytv1.LogWriterType `yson:"type,omitempty"`
	FileName   string             `yson:"file_name,omitempty"`
	Format     ytv1.LogFormat     `yson:"format,omitempty"`

	CompressionMethod    string `yson:"compression_method,omitempty"`
	EnableCompression    bool   `yson:"enable_compression,omitempty"`
	UseTimestampSuffix   bool   `yson:"use_timestamp_suffix,omitempty"`
	EnableSystemMessages bool   `yson:"enable_system_messages,omitempty"`

	RotationPolicy *ytv1.LogRotationPolicy `yson:"rotation_policy,omitempty"`
}

type Logging struct {
	Writers map[string]LoggingWriter `yson:"writers"`
	Rules   []LoggingRule            `yson:"rules"`

	FlushPeriod int `yson:"flush_period"`
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

func newJobProxyLoggingBuilder() loggingBuilder {
	return loggingBuilder{
		loggingDirectory: "",
		componentName:    "job-proxy",
		logging: Logging{
			Rules:   make([]LoggingRule, 0),
			Writers: make(map[string]LoggingWriter),
		},
	}
}

func createBaseLoggingRule(spec ytv1.BaseLoggerSpec) LoggingRule {
	return LoggingRule{
		MinLevel: spec.MinLogLevel,
		Writers:  []string{spec.Name},
	}
}

func createLoggingRule(spec ytv1.TextLoggerSpec) LoggingRule {
	loggingRule := createBaseLoggingRule(spec.BaseLoggerSpec)

	loggingRule.Family = ptr.To(LogFamilyPlainText)

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

func createStructuredLoggingRule(spec ytv1.StructuredLoggerSpec) LoggingRule {
	loggingRule := createBaseLoggingRule(spec.BaseLoggerSpec)
	loggingRule.Family = ptr.To(LogFamilyStructured)
	loggingRule.IncludeCategories = []string{spec.Category}

	return loggingRule
}

func createBaseLoggingWriter(componentName string, loggingDirectory string, writerType ytv1.LogWriterType, loggerSpec ytv1.BaseLoggerSpec) LoggingWriter {
	loggingWriter := LoggingWriter{}

	loggingWriter.WriterType = writerType
	loggingWriter.Format = loggerSpec.Format

	if loggingWriter.WriterType == ytv1.LogWriterTypeFile {
		loggingWriter.FileName = path.Join(loggingDirectory, fmt.Sprintf("%s.%s.log", componentName, loggerSpec.Name))
	}

	if loggingWriter.Format != ytv1.LogFormatPlainText {
		loggingWriter.FileName += fmt.Sprintf(".%s", loggingWriter.Format)
	}

	if loggerSpec.Compression != ytv1.LogCompressionNone {
		loggingWriter.EnableCompression = true
		loggingWriter.CompressionMethod = string(loggerSpec.Compression)
		loggingWriter.FileName += fmt.Sprintf(".%s", loggingWriter.CompressionMethod)
	} else {
		loggingWriter.EnableCompression = false
	}

	loggingWriter.UseTimestampSuffix = loggerSpec.UseTimestampSuffix
	loggingWriter.RotationPolicy = loggerSpec.RotationPolicy
	return loggingWriter
}

func createLoggingWriter(componentName string, loggingDirectory string, loggerSpec ytv1.TextLoggerSpec) LoggingWriter {
	loggingWriter := createBaseLoggingWriter(componentName, loggingDirectory, loggerSpec.WriterType, loggerSpec.BaseLoggerSpec)
	loggingWriter.EnableSystemMessages = true
	return loggingWriter
}

func createStructuredLoggingWriter(componentName string, loggingDirectory string, loggerSpec ytv1.StructuredLoggerSpec) LoggingWriter {
	loggingWriter := createBaseLoggingWriter(componentName, loggingDirectory, ytv1.LogWriterTypeFile, loggerSpec.BaseLoggerSpec)
	loggingWriter.EnableSystemMessages = false
	return loggingWriter
}

func (b *loggingBuilder) addLogger(loggerSpec ytv1.TextLoggerSpec) *loggingBuilder {
	b.logging.Rules = append(b.logging.Rules, createLoggingRule(loggerSpec))
	b.logging.Writers[loggerSpec.Name] = createLoggingWriter(b.componentName, b.loggingDirectory, loggerSpec)

	return b
}

func (b *loggingBuilder) addStructuredLogger(loggerSpec ytv1.StructuredLoggerSpec) *loggingBuilder {
	b.logging.Rules = append(b.logging.Rules, createStructuredLoggingRule(loggerSpec))
	b.logging.Writers[loggerSpec.Name] = createStructuredLoggingWriter(b.componentName, b.loggingDirectory, loggerSpec)

	return b
}
