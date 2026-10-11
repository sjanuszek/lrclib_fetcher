package arguments

import "github.com/spf13/pflag"

var (
	inputPtr string
	mp3Ptr, verbosePtr, debugPtr, SkipLrcPtr, SkipTxtPtr bool
	jobsPtr, fetchJobsPtr, maxRetriesPtr int
)

func GetConfig() Config {
	pflag.StringVarP(&inputPtr, "input", "i", "", "input")

	pflag.IntVarP(&jobsPtr, "jobs", "j", 1, "jobs")

	pflag.IntVarP(&fetchJobsPtr, "fetch-jobs", "f", 4, "fetch jobs")

	pflag.IntVarP(&maxRetriesPtr, "max-retries", "r", 3, "max retries")

	pflag.BoolVarP(&mp3Ptr, "mp3", "m", false, "include mp3")

	pflag.BoolVarP(&verbosePtr, "verbose", "v", false, "verbose")

	pflag.BoolVarP(&debugPtr, "debug", "d", false, "debug")

	pflag.BoolVarP(&SkipLrcPtr, "skip lrc", "l", true, "skip txt")

	pflag.BoolVarP(&SkipTxtPtr, "skip txt", "t", false, "skip txt")

	pflag.Parse()

	return Config{
		inputPtr,
		jobsPtr,
		fetchJobsPtr,
		maxRetriesPtr,
		mp3Ptr,
		verbosePtr,
		debugPtr,
		SkipLrcPtr,
		SkipTxtPtr,
	}
}
