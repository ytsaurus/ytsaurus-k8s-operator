package testutil

import (
	"fmt"
	"math"
	"os"
	"slices"
	"strings"
	"time"

	"golang.org/x/sys/unix"

	"github.com/onsi/ginkgo/v2"
	gtypes "github.com/onsi/ginkgo/v2/types"
	"github.com/onsi/gomega"
	"github.com/onsi/gomega/gmeasure"
)

const (
	TEST_RESULTS_HISTORY_VERSION = 1
)

func getResultsHistory() gmeasure.ExperimentCache {
	cache, err := gmeasure.NewExperimentCache(GetenvOr("TEST_RESULTS_HISTORY", "test-results-history"))
	gomega.Expect(err).To(gomega.Succeed())
	return cache
}

func lockCache(cache gmeasure.ExperimentCache, shared bool) func() {
	f, err := os.Open(cache.Path)
	gomega.Expect(err).To(gomega.Succeed())
	how := unix.LOCK_EX
	if shared {
		how = unix.LOCK_SH
	}
	err = unix.Flock(int(f.Fd()), how)
	gomega.Expect(err).To(gomega.Succeed())
	return func() {
		defer f.Close() //nolint:errcheck //ok
		err := unix.Flock(int(f.Fd()), unix.LOCK_UN)
		gomega.Expect(err).To(gomega.Succeed())
	}
}

func getNodeName(containerHierarchyTexts []string, leafNodeText string) string {
	return strings.Join(containerHierarchyTexts, " / ") + " / " + leafNodeText
}

func getOrigin() string {
	if os.Getenv("GITHUB_RUN_ID") != "" {
		return os.ExpandEnv("$GITHUB_SERVER_URL/$GITHUB_REPOSITORY/actions/runs/$GITHUB_RUN_ID/attempts/$GITHUB_RUN_ATTEMPT")
	}
	hostname, err := os.Hostname()
	gomega.Expect(err).To(gomega.Succeed())
	wd, err := os.Getwd()
	gomega.Expect(err).To(gomega.Succeed())
	return fmt.Sprintf("%v@%v:%v", os.Getenv("USER"), hostname, wd)
}

func UpdateResultsHistory(ctx ginkgo.SpecContext, report ginkgo.SpecReport) {
	cache := getResultsHistory()
	defer lockCache(cache, false)()

	name := getNodeName(report.ContainerHierarchyTexts, report.LeafNodeText)
	record := cache.Load(name, TEST_RESULTS_HISTORY_VERSION)
	if record == nil {
		record = gmeasure.NewExperiment(name)
	} else {
		ginkgo.AddReportEntry("Results history", record)
	}

	if suiteConfig, _ := ginkgo.GinkgoConfiguration(); suiteConfig.DryRun || report.State.Is(gtypes.SpecStateInterrupted) {
		return
	}

	var passed float64
	runTimeMetric := "FailedRunTime"
	if !report.Failed() {
		passed = 1
		runTimeMetric = "PassedRunTime"
	}
	record.RecordValue("Passed", passed, gmeasure.Units("bool"), gmeasure.Annotation(getOrigin()))
	record.RecordDuration(runTimeMetric, report.RunTime, gmeasure.Annotation("StartTime: "+report.StartTime.Format(time.DateTime)))

	if err := cache.Delete(name); err != nil {
		ginkgo.GinkgoLogr.Error(err, "Cannot delete previous result")
	}
	if err := cache.Save(name, TEST_RESULTS_HISTORY_VERSION, record); err != nil {
		ginkgo.GinkgoLogr.Error(err, "Cannot save result")
	}
}

func ApplyResultsHistory(nodeType gtypes.NodeType, offset ginkgo.Offset, text string, args []any) (t string, a []any, e []error) {
	if nodeType == gtypes.NodeTypeIt {
		tree := ginkgo.CurrentTreeConstructionNodeReport()
		name := getNodeName(tree.ContainerHierarchyTexts[1:], text)
		cache := getResultsHistory()
		defer lockCache(cache, true)()
		if record := cache.Load(name, TEST_RESULTS_HISTORY_VERSION); record != nil {
			if passed := record.Get("Passed").Values; slices.Contains(passed, 0) {
				n := len(passed)
				var priority float64
				// Increase priority for recently failed tests.
				for i, p := range passed {
					priority += 1000 * (1 - p) * math.Exp2(float64(i-n))
				}
				// Increase priority for failed fast tests.
				failedRunTime := record.GetStats("FailedRunTime").DurationFor(gmeasure.StatMedian)
				priority += 1000 / max(failedRunTime, 10*time.Second).Seconds()
				// Add label and adjust priority for recently failed tests.
				args = append(args, ginkgo.SpecPriority(int(priority)), ginkgo.Label("recently-failed"))
			}
		}
	}
	return text, args, nil
}

func SetupResultsHistory() {
	ginkgo.ReportAfterEach(UpdateResultsHistory)
	ginkgo.AddTreeConstructionNodeArgsTransformer(ApplyResultsHistory)
}
