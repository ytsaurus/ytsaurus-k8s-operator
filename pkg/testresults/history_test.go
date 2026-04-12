package testresults

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestEMA(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Test Results")
}

var _ = Context("Test Results", func() {
	It("Tests EMA", func() {
		Expect(expMovingAverage(nil)).To(Equal(0.0))
		Expect(expMovingAverage([]float64{0, 0, 0, 0, 0})).To(BeNumerically("<", 0.9))
		Expect(expMovingAverage([]float64{1, 1, 1, 1, 0})).To(BeNumerically("<", 0.9))
		Expect(expMovingAverage([]float64{1, 1, 1, 0, 1})).To(BeNumerically("<", 0.9))
		Expect(expMovingAverage([]float64{1, 1, 0, 1, 1})).To(BeNumerically("<", 0.9))
		Expect(expMovingAverage([]float64{0, 0, 1, 1, 1})).To(BeNumerically("<", 0.9))
		Expect(expMovingAverage([]float64{1, 0, 1, 1, 1})).To(BeNumerically(">", 0.9))
		Expect(expMovingAverage([]float64{0, 1, 1, 1, 1})).To(BeNumerically(">", 0.9))
		Expect(expMovingAverage([]float64{1, 1, 1, 1, 1})).To(BeNumerically(">", 0.9))
	})
})
