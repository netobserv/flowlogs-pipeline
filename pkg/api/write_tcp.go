package api

type WriteTCP struct {
	Port string `yaml:"port,omitempty" json:"port,omitempty" doc:"TCP port number"`
}
