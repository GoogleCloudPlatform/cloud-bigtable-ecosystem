package common

import (
	"fmt"
	"github.com/GoogleCloudPlatform/cloud-bigtable-ecosystem/cassandra-bigtable-migration-tools/cassandra-bigtable-proxy/global/types"
	cql "github.com/GoogleCloudPlatform/cloud-bigtable-ecosystem/cassandra-bigtable-migration-tools/cassandra-bigtable-proxy/third_party/cqlparser"
	"strings"
)

func ParseCqlFunc(f cql.IFunctionCallContext) (types.CqlFuncCode, error) {
	if f.K_UUID() != nil {
		return types.FuncCodeUnknown, fmt.Errorf("unknown function: 'UUID'")
	}
	// writetime is a reserved word, so we have to handle that case separately
	if f.K_WRITETIME() != nil {
		return types.FuncCodeWriteTime, nil
	}

	functionName := strings.ToLower(f.OBJECT_NAME().GetText())
	switch functionName {
	case "writetime":
		return types.FuncCodeWriteTime, nil
	case "count":
		return types.FuncCodeCount, nil
	case "avg":
		return types.FuncCodeAvg, nil
	case "sum":
		return types.FuncCodeSum, nil
	case "min":
		return types.FuncCodeMin, nil
	case "max":
		return types.FuncCodeMax, nil
	case "now":
		return types.FuncCodeNow, nil
	case "totimestamp":
		return types.FuncCodeToTimestamp, nil
	default:
		return types.FuncCodeUnknown, fmt.Errorf("unknown function: '%s'", functionName)
	}
}
