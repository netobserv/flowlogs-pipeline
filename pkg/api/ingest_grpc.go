package api

type IngestGRPCProto struct {
	Port      int `yaml:"port" doc:"the port number to listen on"`
	BufferLen int `yaml:"buffer_length" doc:"the length of the ingest channel buffer, in groups of flows, containing each group hundreds of flows (default: 100)"`
}
