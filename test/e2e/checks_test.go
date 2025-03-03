package controllers_test

func getInitializingStageJobNames() []string {
	return []string{
		"yt-master-init-job-default",
		"yt-client-init-job-user",
		"yt-scheduler-init-job-user",
		"yt-scheduler-init-job-op-archive",
	}
}
