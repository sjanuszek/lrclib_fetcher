package arguments

import (
	"fmt"
	"os"
	"runtime"
)

type Config struct {
	InputPath string
	Jobs int
	FetchJobs int
	MaxRetries int
	ParseMP3 bool
	Verbose bool
	Debug bool
	NoSkip bool
}

func (cfg *Config) VerifyConfig() error {
	if cfg.InputPath == "" {
		return fmt.Errorf("Input path can't be empty")
	} 

	info, err := os.Stat(cfg.InputPath)

	if err != nil {
		return fmt.Errorf("Input path error")
	}

	if !info.IsDir() {
		return fmt.Errorf("Input path is not a directory")
	}

	if cfg.Jobs <= 0 || cfg.Jobs > runtime.NumCPU() {
		return fmt.Errorf("Jobs count must be between (%d, %d]", 0, runtime.NumCPU())
	}

	if cfg.FetchJobs <= 0 {
		return fmt.Errorf("Fetch jobs count must be more than 0")
	}

	if cfg.MaxRetries <= 0 {
		return fmt.Errorf("Max retries count must be more than 0")
	}

	return nil
}
