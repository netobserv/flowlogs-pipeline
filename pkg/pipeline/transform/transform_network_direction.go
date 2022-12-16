package transform

import (
	"github.com/netobserv/flowlogs-pipeline/pkg/api"
	"github.com/netobserv/flowlogs-pipeline/pkg/config"
)

const (
	ingress = 0
	egress  = 1
)

func reinterpretDirection(output config.GenericMap, info *api.DirectionInfo) {
	output[info.IfDirectionField] = output[info.FlowDirectionField]
	var srcNode, dstNode, reporter string
	if gen, ok := output[info.SrcHostField]; ok {
		if str, ok := gen.(string); ok {
			srcNode = str
		}
	}
	if gen, ok := output[info.DstHostField]; ok {
		if str, ok := gen.(string); ok {
			dstNode = str
		}
	}
	if gen, ok := output[info.ReporterIPField]; ok {
		if str, ok := gen.(string); ok {
			reporter = str
		}
	}
	if srcNode != dstNode {
		if srcNode == reporter {
			output[info.FlowDirectionField] = egress
		} else if dstNode == reporter {
			output[info.FlowDirectionField] = ingress
		}
	}
}
