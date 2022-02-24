package api

type AggregateBy []string
type AggregateOperation string

type AggregateDefinition struct {
	Name      string
	By        AggregateBy
	Operation AggregateOperation
	RecordKey string
}
