package controllers

import (
	"github.com/ytsaurus/yt-k8s-operator/pkg/flows"
)

type baseStep struct {
	name flows.StepName
}

func (s *baseStep) StepName() flows.StepName {
	return s.name
}
