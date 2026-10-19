package controller

import (
	"github.com/harvester/resource-health-controller/pkg/config"
	"github.com/harvester/resource-health-controller/pkg/controller/componenthealth"
	"github.com/harvester/resource-health-controller/pkg/controller/healthsummary"
)

var RegisterFuncList = []config.RegisterFunc{
	componenthealth.Register,
	healthsummary.Register,
}
