package controllers

import (
	"github.com/ytsaurus/yt-k8s-operator/pkg/components"
	"github.com/ytsaurus/yt-k8s-operator/pkg/flows"
)

func newJobStep(job components.InitJob) flows.ActionStep {
	return flows.ActionStep{
		Name:        flows.StepName(job.GetName()),
		PreRunFunc:  nil,
		RunFunc:     nil,
		PostRunFunc: nil,
	}
}
