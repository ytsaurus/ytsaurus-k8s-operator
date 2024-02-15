package controllers

type dummyStep struct {
	name string
}

func newDummyStep() *dummyStep {
	return &dummyStep{}
}
