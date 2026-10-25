package validators

import (
	"testing"

	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/utils/ptr"

	ytv1 "github.com/ytsaurus/ytsaurus-k8s-operator/api/v1"
)

func TestValidateLoggerSuffixes(t *testing.T) {
	for _, testCase := range []struct {
		name         string
		writerType   ytv1.LogWriterType
		timestamp    bool
		dateTime     bool
		wantErrors   int
		wantWarnings int
	}{
		{name: "file without suffix", writerType: ytv1.LogWriterTypeFile},
		{name: "file timestamp suffix", writerType: ytv1.LogWriterTypeFile, timestamp: true, wantWarnings: 1},
		{name: "file date time suffix", writerType: ytv1.LogWriterTypeFile, dateTime: true},
		{name: "file conflicting suffixes", writerType: ytv1.LogWriterTypeFile, timestamp: true, dateTime: true, wantErrors: 1},
		{name: "stderr without suffix", writerType: ytv1.LogWriterTypeStderr},
		{name: "stderr timestamp suffix", writerType: ytv1.LogWriterTypeStderr, timestamp: true},
		{name: "stderr date time suffix", writerType: ytv1.LogWriterTypeStderr, dateTime: true},
		{name: "stderr conflicting suffixes", writerType: ytv1.LogWriterTypeStderr, timestamp: true, dateTime: true},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			logger := ytv1.TextLoggerSpec{
				BaseLoggerSpec: ytv1.BaseLoggerSpec{
					Name:               "test",
					MinLogLevel:        ytv1.LogLevelInfo,
					UseTimestampSuffix: testCase.timestamp,
					UseDateTimeSuffix:  testCase.dateTime,
				},
				WriterType: testCase.writerType,
			}
			if logger.WriterType == ytv1.LogWriterTypeFile {
				logger.RotationPolicy = &ytv1.LogRotationPolicy{
					MaxTotalSizeToKeep: ptr.To(resource.MustParse("100Mi")),
					MaxSegmentSize:     ptr.To(resource.MustParse("10Mi")),
				}
			}
			results := validateLoggers([]ytv1.TextLoggerSpec{logger}, field.NewPath("spec", "discovery", "loggers"))
			require.Len(t, results.Errors, testCase.wantErrors)
			require.Len(t, results.Warnings, testCase.wantWarnings)
			if testCase.wantErrors > 0 {
				require.Equal(t, "spec.discovery.loggers[0].useTimestampSuffix", results.Errors[0].Field)
			}
			if testCase.wantWarnings > 0 {
				require.Contains(t, results.Warnings[0].Error(), "spec.discovery.loggers[0].useTimestampSuffix")
			}
		})
	}
}

func TestValidateLoggersNoisyCategories(t *testing.T) {
	for _, testCase := range []struct {
		name        string
		filter      *ytv1.CategoriesFilter
		wantWarning bool
	}{
		{name: "no filter", wantWarning: true},
		{
			name:        "excludes Bus only",
			filter:      &ytv1.CategoriesFilter{Type: ytv1.CategoriesFilterTypeExclude, Values: []string{"Bus"}},
			wantWarning: true,
		},
		{
			name:        "excludes Concurrency only",
			filter:      &ytv1.CategoriesFilter{Type: ytv1.CategoriesFilterTypeExclude, Values: []string{"Concurrency"}},
			wantWarning: true,
		},
		{
			name:   "excludes both noisy categories",
			filter: &ytv1.CategoriesFilter{Type: ytv1.CategoriesFilterTypeExclude, Values: []string{"Bus", "Concurrency"}},
		},
		{
			name:        "includes Bus",
			filter:      &ytv1.CategoriesFilter{Type: ytv1.CategoriesFilterTypeInclude, Values: []string{"Bus"}},
			wantWarning: true,
		},
		{
			name:        "includes Concurrency",
			filter:      &ytv1.CategoriesFilter{Type: ytv1.CategoriesFilterTypeInclude, Values: []string{"Concurrency"}},
			wantWarning: true,
		},
		{
			name:        "includes both noisy categories",
			filter:      &ytv1.CategoriesFilter{Type: ytv1.CategoriesFilterTypeInclude, Values: []string{"Bus", "Concurrency"}},
			wantWarning: true,
		},
		{
			name:   "includes another category",
			filter: &ytv1.CategoriesFilter{Type: ytv1.CategoriesFilterTypeInclude, Values: []string{"Scheduler"}},
		},
		{
			name:        "empty include list does not restrict categories",
			filter:      &ytv1.CategoriesFilter{Type: ytv1.CategoriesFilterTypeInclude},
			wantWarning: true,
		},
		{
			name:        "missing filter type does not restrict categories",
			filter:      &ytv1.CategoriesFilter{Values: []string{"Bus", "Concurrency"}},
			wantWarning: true,
		},
	} {
		for _, level := range []ytv1.LogLevel{ytv1.LogLevelDebug, ytv1.LogLevelTrace} {
			t.Run(testCase.name+"/"+string(level), func(t *testing.T) {
				logger := ytv1.TextLoggerSpec{
					BaseLoggerSpec: ytv1.BaseLoggerSpec{
						Name:        "test",
						MinLogLevel: level,
						Compression: ytv1.LogCompressionZstd,
						RotationPolicy: &ytv1.LogRotationPolicy{
							MaxTotalSizeToKeep: ptr.To(resource.MustParse("100Mi")),
							MaxSegmentSize:     ptr.To(resource.MustParse("10Mi")),
						},
					},
					WriterType:       ytv1.LogWriterTypeFile,
					CategoriesFilter: testCase.filter,
				}
				results := validateLoggers([]ytv1.TextLoggerSpec{logger}, field.NewPath("spec", "discovery", "loggers"))
				require.Empty(t, results.Errors)
				if testCase.wantWarning {
					require.Len(t, results.Warnings, 1)
					require.Contains(t, results.Warnings[0].Error(), "spec.discovery.loggers[0].categoriesFilter")
				} else {
					require.Empty(t, results.Warnings)
				}
			})
		}
	}
}
