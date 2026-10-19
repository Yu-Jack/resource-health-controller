package main

import (
	"os"

	controllergen "github.com/rancher/wrangler/v3/pkg/controller-gen"
	"github.com/rancher/wrangler/v3/pkg/controller-gen/args"
)

func main() {
	_ = os.Unsetenv("GOPATH")
	// Only the CRDs owned by this addon are generated here. Controllers for the
	// resources being watched (Node, VMI, Longhorn Volume, ...) are imported from
	// github.com/harvester/harvester/pkg/generated.
	controllergen.Run(args.Options{
		OutputPackage: "github.com/harvester/resource-health-controller/pkg/generated",
		Boilerplate:   "scripts/boilerplate.go.txt",
		Groups: map[string]args.Group{
			"health.harvesterhci.io": {
				Types: []interface{}{
					"./pkg/apis/health.harvesterhci.io/v1beta1",
				},
				GenerateTypes:   true,
				GenerateClients: true,
			},
		},
	})
}
