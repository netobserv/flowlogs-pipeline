package api

type AggregateBy []string
type AggregateOperation string

type AggregateDefinition struct {
	Name      string             `yaml:"Name" doc:"description of aggregation result"`
	By        AggregateBy        `yaml:"By" doc:"list of fields on which to aggregate"`
	Operation AggregateOperation `yaml:"Operation" doc:"sum, min, max, avg and nop"`
	RecordKey string             `yaml:"RecordKey" doc:"internal field on which to perform the operation"`
}
