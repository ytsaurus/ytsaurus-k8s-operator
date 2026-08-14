package ytconfig

import (
	"fmt"
	"path"
	"time"

	"go.ytsaurus.tech/yt/go/yson"
	"k8s.io/utils/ptr"

	ytv1 "github.com/ytsaurus/ytsaurus-k8s-operator/api/v1"
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

func defaultClientStderrLoggerSpec() ytv1.TextLoggerSpec {
	return ytv1.TextLoggerSpec{
		BaseLoggerSpec: ytv1.BaseLoggerSpec{
			Name:               "stderr",
			MinLogLevel:        ytv1.LogLevelWarning,
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

type LogRotationPolicy struct {
	RotationPeriodMilliseconds *int64 `yson:"rotation_period,omitempty"`
	MaxSegmentSize             *int64 `yson:"max_segment_size,omitempty"`
	MaxTotalSizeToKeep         *int64 `yson:"max_total_size_to_keep,omitempty"`
	MaxSegmentCountToKeep      *int64 `yson:"max_segment_count_to_keep,omitempty"`
}

type LoggingWriter struct {
	WriterType ytv1.LogWriterType `yson:"type,omitempty"`
	FileName   string             `yson:"file_name,omitempty"`
	Format     ytv1.LogFormat     `yson:"format,omitempty"`

	CompressionMethod    string `yson:"compression_method,omitempty"`
	EnableCompression    bool   `yson:"enable_compression,omitempty"`
	UseTimestampSuffix   bool   `yson:"use_timestamp_suffix,omitempty"`
	EnableSystemMessages bool   `yson:"enable_system_messages,omitempty"`

	RotationPolicy *LogRotationPolicy `yson:"rotation_policy,omitempty"`
}

type Logging struct {
	Writers map[string]LoggingWriter `yson:"writers"`
	Rules   []LoggingRule            `yson:"rules"`

	FlushPeriod           int  `yson:"flush_period,omitempty"`
	EnableAnchorProfiling bool `yson:"enable_anchor_profiling,omitempty"`
}

type JobProxyLogging struct {
	// COMPAT(ignat)
	// 23.2 — job_proxy_logging
	// 24.1 — job_proxy/job_proxy_logging
	// 24.2 — job_proxy/job_proxy_logging/log_manager_template
	// Legacy fields can be removed with end of respective server version support.
	Logging
	LogManagerTemplate Logging `yson:"log_manager_template"`
	Mode               string  `yson:"mode"`
}

type loggingBuilder struct {
	loggingDirectory string
	componentName    string
	logging          Logging
}

func ChooseLoggingPath(location *ytv1.LocationSpec) string {
	loggingDirectory := "/var/log"
	if location != nil {
		loggingDirectory = location.Path
	}
	return loggingDirectory
}

func ChooseJobProxyLoggingPath(spec *ytv1.InstanceSpec) string {
	if location := ytv1.FindFirstLocation(spec.Locations, ytv1.LocationTypeLogs); location != nil {
		return location.Path + "/job-proxy"
	}
	return "/var/log/job-proxy"
}

const JobProxyLogSymlinksPath = "/var/log/job-proxy-logs"

func newLoggingBuilder(location *ytv1.LocationSpec, componentName string) loggingBuilder {
	loggingDirectory := ChooseLoggingPath(location)

	return loggingBuilder{
		loggingDirectory: loggingDirectory,
		componentName:    componentName,
		logging: Logging{
			Rules:   make([]LoggingRule, 0),
			Writers: make(map[string]LoggingWriter),
		},
	}
}

type jobProxyLoggingBuilder struct {
	loggingBuilder
	spec *ytv1.ExecNodesSpec
}

func newJobProxyLoggingBuilder(spec *ytv1.ExecNodesSpec) jobProxyLoggingBuilder {
	builder := jobProxyLoggingBuilder{
		loggingBuilder: loggingBuilder{
			componentName: "job-proxy",
			logging: Logging{
				Rules:   make([]LoggingRule, 0),
				Writers: make(map[string]LoggingWriter),
			},
		},
		spec: spec,
	}
	if len(spec.JobProxyLoggers) > 0 {
		for _, loggerSpec := range spec.JobProxyLoggers {
			builder.addLogger(loggerSpec)
		}
	} else {
		defaultLoggerSpecs := []ytv1.TextLoggerSpec{defaultInfoLoggerSpec(), defaultStderrLoggerSpec()}
		if builder.getMode() == ytv1.JobProxyLoggingModePerJobDirectory {
			defaultLoggerSpecs = append(defaultLoggerSpecs, defaultDebugLoggerSpec())
		}
		for _, defaultLoggerSpec := range defaultLoggerSpecs {
			builder.addLogger(defaultLoggerSpec)
		}
	}
	builder.logging.FlushPeriod = 3000
	return builder
}

func (builder *jobProxyLoggingBuilder) getMode() ytv1.JobProxyLoggingMode {
	if builder.spec.JobProxyLogManager != nil && builder.spec.JobProxyLogManager.Mode != "" {
		return builder.spec.JobProxyLogManager.Mode
	}
	return ytv1.JobProxyLoggingModeSimple
}

func (builder *jobProxyLoggingBuilder) buildJobProxyLogging() JobProxyLogging {
	return JobProxyLogging{
		Logging:            builder.logging,
		LogManagerTemplate: builder.logging,
		Mode:               string(builder.getMode()),
	}
}

func (builder *jobProxyLoggingBuilder) buildJobProxyLogManager() JobProxyLogManager {
	logsStoragePeriod := yson.Duration(7 * 24 * time.Hour)
	directoryTraversalConcurrency := 4
	jobProxyLogSymlinksPath := JobProxyLogSymlinksPath
	if builder.spec.JobProxyLogManager != nil {
		if builder.spec.JobProxyLogManager.LogsStoragePeriodMilliseconds != nil {
			logsStoragePeriod = yson.Duration(time.Duration(*builder.spec.JobProxyLogManager.LogsStoragePeriodMilliseconds) * time.Millisecond)
		}
		if builder.spec.JobProxyLogManager.DirectoryTraversalConcurrency != nil {
			directoryTraversalConcurrency = *builder.spec.JobProxyLogManager.DirectoryTraversalConcurrency
		}
		if builder.spec.JobProxyLogManager.JobProxyLogSymlinksPath != nil {
			jobProxyLogSymlinksPath = *builder.spec.JobProxyLogManager.JobProxyLogSymlinksPath
		}
	}
	logManager := JobProxyLogManager{
		ShardingKeyLength:             2,
		LogsStoragePeriod:             logsStoragePeriod,
		DirectoryTraversalConcurrency: directoryTraversalConcurrency,
		LogDump: LogDump{
			BufferSize:    1024 * 1024,
			LogWriterName: "debug",
		},
	}

	if builder.getMode() == ytv1.JobProxyLoggingModePerJobDirectory {
		for _, location := range ytv1.FindAllLocations(builder.spec.Locations, ytv1.LocationTypeJobProxyLogs) {
			logManager.Locations = append(
				logManager.Locations,
				JobProxyLogManagerLocation{Path: location.Path},
			)
		}
	}

	if len(logManager.Locations) > 0 {
		// multi-location mode accessible >= 26.1
		logManager.JobProxyLogSymlinksPath = jobProxyLogSymlinksPath
		// COMPAT(epsilond1): Remove after e2e uses >= 26.1
		logManager.Directory = logManager.Locations[0].Path
	} else {
		logManager.Directory = ChooseJobProxyLoggingPath(&builder.spec.InstanceSpec)
	}
	return logManager
}
func createBaseLoggingRule(spec ytv1.BaseLoggerSpec) LoggingRule {
	return LoggingRule{
		MinLevel: spec.MinLogLevel,
		Writers:  []string{spec.Name},
	}
}

func applyCategoriesFilter(loggingRule *LoggingRule, filter *ytv1.CategoriesFilter) {
	if filter == nil {
		return
	}

	switch filter.Type {
	case ytv1.CategoriesFilterTypeExclude:
		loggingRule.ExcludeCategories = append(loggingRule.ExcludeCategories, filter.Values...)

	case ytv1.CategoriesFilterTypeInclude:
		loggingRule.IncludeCategories = append(loggingRule.IncludeCategories, filter.Values...)
	}
}

func createLoggingRule(spec ytv1.TextLoggerSpec) LoggingRule {
	loggingRule := createBaseLoggingRule(spec.BaseLoggerSpec)

	loggingRule.Family = ptr.To(LogFamilyPlainText)

	applyCategoriesFilter(&loggingRule, spec.CategoriesFilter)
	return loggingRule
}

func createStructuredLoggingRule(spec ytv1.StructuredLoggerSpec) LoggingRule {
	loggingRule := createBaseLoggingRule(spec.BaseLoggerSpec)
	loggingRule.Family = ptr.To(LogFamilyStructured)

	if spec.CategoriesFilter != nil {
		applyCategoriesFilter(&loggingRule, spec.CategoriesFilter)
	} else {
		loggingRule.IncludeCategories = []string{ptr.Deref(spec.Category, "")}
	}
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

	if loggerSpec.RotationPolicy != nil {
		loggingWriter.RotationPolicy = &LogRotationPolicy{
			RotationPeriodMilliseconds: loggerSpec.RotationPolicy.RotationPeriodMilliseconds,
			MaxSegmentCountToKeep:      loggerSpec.RotationPolicy.MaxSegmentCountToKeep,
		}
		if loggerSpec.RotationPolicy.MaxSegmentSize != nil {
			loggingWriter.RotationPolicy.MaxSegmentSize = ptr.To(loggerSpec.RotationPolicy.MaxSegmentSize.Value())
		}
		if loggerSpec.RotationPolicy.MaxTotalSizeToKeep != nil {
			loggingWriter.RotationPolicy.MaxTotalSizeToKeep = ptr.To(loggerSpec.RotationPolicy.MaxTotalSizeToKeep.Value())
		}
	}
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
