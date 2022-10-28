package network

import (
	"fmt"
	"net"
	"regexp"
	"strconv"

	"github.com/Knetic/govaluate"
	"github.com/netobserv/flowlogs-pipeline/pkg/api"
	"github.com/netobserv/flowlogs-pipeline/pkg/config"
	"github.com/netobserv/flowlogs-pipeline/pkg/pipeline/transform/kubernetes"
	"github.com/netobserv/flowlogs-pipeline/pkg/pipeline/transform/location"
	"github.com/netobserv/flowlogs-pipeline/pkg/pipeline/transform/netdb"
)

type Transformer interface {
	Transform(flow config.GenericMap) error
}

type AddRegexpIf api.NetworkTransformRule

func (ari *AddRegexpIf) Transform(flow config.GenericMap) error {
	matched, err := regexp.MatchString(ari.Parameters, fmt.Sprintf("%s", flow[ari.Input]))
	if err != nil {
		return fmt.Errorf("matching regexp %q: %w", ari.Parameters, err)
	}
	if matched {
		flow[ari.Output] = flow[ari.Input]
		flow[ari.Output+"_Matched"] = true
	}
	return nil
}

type AddIf api.NetworkTransformRule

func (ai *AddIf) Transform(flow config.GenericMap) error {
	expressionString := fmt.Sprintf("val %s", ai.Parameters)
	expression, err := govaluate.NewEvaluableExpression(expressionString)
	if err != nil {
		return fmt.Errorf("can't evaluate AddIf ai: %+v expression: %v. err %v",
			ai, expressionString, err)
	}
	result, evaluateErr := expression.Evaluate(map[string]interface{}{"val": flow[ai.Input]})
	if evaluateErr == nil && result.(bool) {
		if ai.Assignee != "" {
			flow[ai.Output] = ai.Assignee
		} else {
			flow[ai.Output] = flow[ai.Input]
		}
		flow[ai.Output+"_Evaluate"] = true
	}
	return nil
}

type AddSubnet api.NetworkTransformRule

func (as *AddSubnet) Transform(flow config.GenericMap) error {
	_, ipv4Net, err := net.ParseCIDR(fmt.Sprintf("%v%s", flow[as.Input], as.Parameters))
	if err != nil {
		return fmt.Errorf("can't find subnet for IP %v and prefix length %s - err %v", flow[as.Input], as.Parameters, err)
	}
	flow[as.Output] = ipv4Net.String()
	return nil
}

type AddLocation api.NetworkTransformRule

func (al *AddLocation) Transform(flow config.GenericMap) error {
	err, locationInfo := location.GetLocation(fmt.Sprintf("%s", flow[al.Input]))
	if err != nil {
		return fmt.Errorf("can't find location for IP %v err %v", flow[al.Input], err)
	}
	flow[al.Output+"_CountryName"] = locationInfo.CountryName
	flow[al.Output+"_CountryLongName"] = locationInfo.CountryLongName
	flow[al.Output+"_RegionName"] = locationInfo.RegionName
	flow[al.Output+"_CityName"] = locationInfo.CityName
	flow[al.Output+"_Latitude"] = locationInfo.Latitude
	flow[al.Output+"_Longitude"] = locationInfo.Longitude

	return nil
}

type AddService struct {
	api.NetworkTransformRule
	SvcNames *netdb.ServiceNames
}

func (as *AddService) Transform(flow config.GenericMap) error {
	protocol := fmt.Sprintf("%v", flow[as.Parameters])
	portNumber, err := strconv.Atoi(fmt.Sprintf("%v", flow[as.Input]))
	if err != nil {
		return fmt.Errorf("can't convert port to int: Port %v - err %v", flow[as.Input], err)
	}
	var serviceName string
	protocolAsNumber, err := strconv.Atoi(protocol)
	if err == nil {
		// protocol has been submitted as number
		serviceName = as.SvcNames.ByPortAndProtocolNumber(portNumber, protocolAsNumber)
	} else {
		// protocol has been submitted as any string
		serviceName = as.SvcNames.ByPortAndProtocolName(portNumber, protocol)
	}
	if serviceName == "" {
		if err != nil {
			return fmt.Errorf("can't find service name for Port %v and protocol %v - err %v", flow[as.Input], protocol, err)
		}
	}
	flow[as.Output] = serviceName
	return nil
}

type AddKubernetes api.NetworkTransformRule

func (ak *AddKubernetes) Transform(flow config.GenericMap) error {
	kubeInfo, err := kubernetes.Data.GetInfo(fmt.Sprintf("%s", flow[ak.Input]))
	if err != nil {
		return fmt.Errorf("can't find kubernetes info for IP %v err %v", flow[ak.Input], err)
	}
	flow[ak.Output+"_Namespace"] = kubeInfo.Namespace
	flow[ak.Output+"_Name"] = kubeInfo.Name
	flow[ak.Output+"_Type"] = kubeInfo.Type
	flow[ak.Output+"_OwnerName"] = kubeInfo.Owner.Name
	flow[ak.Output+"_OwnerType"] = kubeInfo.Owner.Type
	if ak.Parameters != "" {
		for labelKey, labelValue := range kubeInfo.Labels {
			flow[ak.Parameters+"_"+labelKey] = labelValue
		}
	}
	if kubeInfo.HostIP != "" {
		flow[ak.Output+"_HostIP"] = kubeInfo.HostIP
		if kubeInfo.HostName != "" {
			flow[ak.Output+"_HostName"] = kubeInfo.HostName
		}
	}
	return nil
}
