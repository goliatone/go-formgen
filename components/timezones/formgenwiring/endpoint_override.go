package formgenwiring

import (
	"strconv"

	"github.com/goliatone/go-formgen/components/timezones"
	formgenorchestrator "github.com/goliatone/go-formgen/pkg/orchestrator"
)

// TimezonesEndpointOverride returns a go-formgen orchestrator EndpointOverride for a relationship-backed
// timezone field, using the timezones component defaults (and any provided overrides).
//
// The generated override:
// - points at <basePath><RoutePath> (default: <basePath>/api/timezones)
// - uses resultsPath "data" with value/label mapping
// - includes "format=options" and a default limit param
// - includes a dynamic search param mapped to "{{self}}"
func TimezonesEndpointOverride(operationID, fieldPath, basePath string, fns ...timezones.OptionFn) formgenorchestrator.EndpointOverride {
	opts := timezones.NewOptions(fns...)
	url := timezones.MountPath(basePath, func(o *timezones.Options) {
		if o == nil {
			return
		}
		*o = opts
	})

	params := map[string]string{
		"format":        "options",
		opts.LimitParam: strconv.Itoa(opts.DefaultLimit),
	}

	return formgenorchestrator.EndpointOverride{
		OperationID: operationID,
		FieldPath:   fieldPath,
		Endpoint: formgenorchestrator.EndpointConfig{
			URL:         url,
			Method:      "GET",
			ResultsPath: "data",
			Params:      params,
			DynamicParams: map[string]string{
				opts.SearchParam: "{{self}}",
			},
			Mapping: formgenorchestrator.EndpointMapping{
				Value: "value",
				Label: "label",
			},
		},
	}
}
