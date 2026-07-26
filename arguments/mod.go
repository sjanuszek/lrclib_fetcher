package arguments

type Config struct {
	InputPath string
	Jobs int
	FetchJobs int
	MaxRetries int
	ParseMP3 bool
	Verbose bool
	Debug bool
}
