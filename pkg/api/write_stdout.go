package api

import "github.com/mariomac/pipes/pkg/graph/stage"

type WriteStdout struct {
	stage.Instance
	Format string `yaml:"format" doc:"the format of each line: printf (default) or json"`
}
