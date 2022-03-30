package api

type WriteStdout struct {
	Format string `yaml:"format" doc:"the format of each line: printf (default) or json"`
}
