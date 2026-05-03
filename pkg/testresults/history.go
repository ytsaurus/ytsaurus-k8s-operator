package testresults

import (
	"fmt"
	"os"
	"strings"
	"time"

	"golang.org/x/sys/unix"

	"github.com/onsi/ginkgo/v2"
	gtypes "github.com/onsi/ginkgo/v2/types"
	"github.com/onsi/gomega"
	"github.com/onsi/gomega/gmeasure"
)

func init() {
	if path := os.Getenv("TEST_RESULTS_HISTORY"); path != "" {
		cache, err := gmeasure.NewExperimentCache(path)
		if err != nil {
			ginkgo.GinkgoLogr.Error(err, "Cannot initialize test results history")
			return
		}
		history := historyCache{ExperimentCache: cache}
		ginkgo.ReportAfterEach(history.saveReport)
		ginkgo.AddTreeConstructionNodeArgsTransformer(history.loadHistory)
	}
}

const (
	historyVersion = 1
)

type historyCache struct {
	gmeasure.ExperimentCache
}

func lockPath(path string, shared bool) func() {
	f, err := os.Open(path) //nolint:gosec //ok
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

func (hc historyCache) saveReport(ctx ginkgo.SpecContext, report ginkgo.SpecReport) {
	defer lockPath(hc.Path, false)()

	name := getNodeName(report.ContainerHierarchyTexts, report.LeafNodeText)
	record := hc.Load(name, historyVersion)
	if record == nil {
		record = gmeasure.NewExperiment(name)
	} else {
		ginkgo.AddReportEntry("Results history", record)
	}

	if suiteConfig, _ := ginkgo.GinkgoConfiguration(); suiteConfig.DryRun || report.State.Is(gtypes.SpecStateInterrupted|gtypes.SpecStateSkipped) {
		return
	}

	var passed float64
	runTimeMetric := "FailedRunTime"
	if !report.Failed() {
		passed = 1
		runTimeMetric = "PassedRunTime"
	}
	record.RecordValue("Passed", passed, gmeasure.Units("bool"), gmeasure.Annotation(getOrigin()))
	record.RecordDuration("RunTime", report.RunTime, gmeasure.Annotation("StartTime: "+report.StartTime.Format(time.DateTime)))
	record.RecordDuration(runTimeMetric, report.RunTime, gmeasure.Annotation("StartTime: "+report.StartTime.Format(time.DateTime)))

	if err := hc.Delete(name); err != nil {
		ginkgo.GinkgoLogr.Error(err, "Cannot delete previous result")
	}
	if err := hc.Save(name, historyVersion, record); err != nil {
		ginkgo.GinkgoLogr.Error(err, "Cannot save result")
	}
}

func expMovingAverage(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	s := values[0]
	for i := 1; i < len(values); i++ {
		s = 0.5 * (values[i] + s)
	}
	return s
}

func expMovingAverageDiration(values []time.Duration) time.Duration {
	if len(values) == 0 {
		return 0
	}
	s := float64(values[0])
	for i := 1; i < len(values); i++ {
		s = 0.5 * (float64(values[i]) + s)
	}
	return time.Duration(s)
}

func (hc historyCache) loadHistory(nodeType gtypes.NodeType, offset ginkgo.Offset, text string, args []any) (t string, a []any, e []error) {
	if nodeType == gtypes.NodeTypeIt {
		defer lockPath(hc.Path, true)()
		tree := ginkgo.CurrentTreeConstructionNodeReport()
		name := getNodeName(tree.ContainerHierarchyTexts[1:], text)
		if record := hc.Load(name, historyVersion); record != nil {
			avgPassed := expMovingAverage(record.Get("Passed").Values)

			// Increase priority for recently failed tests.
			priority := 1000. * (1 - avgPassed)

			// Increase priority for fast tests.
			avgRunTime := expMovingAverageDiration(record.Get("RunTime").Durations)
			priority += 1000. / max(avgRunTime, 10*time.Second).Seconds()

			args = append(args, ginkgo.SpecPriority(int(priority)))

			// Look at past three/four results
			if avgPassed < 0.9 {
				args = append(args, ginkgo.Label("recently-failed"))
			}
		}
	}
	return text, args, nil
}
