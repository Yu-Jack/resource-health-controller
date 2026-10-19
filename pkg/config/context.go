package config

import (
	"context"

	harvesterv1 "github.com/harvester/harvester/pkg/apis/harvesterhci.io/v1beta1"
	ctlcore "github.com/harvester/harvester/pkg/generated/controllers/core"
	ctlharvester "github.com/harvester/harvester/pkg/generated/controllers/harvesterhci.io"
	ctlkubevirt "github.com/harvester/harvester/pkg/generated/controllers/kubevirt.io"
	ctllonghorn "github.com/harvester/harvester/pkg/generated/controllers/longhorn.io"
	longhornv1 "github.com/longhorn/longhorn-manager/k8s/pkg/apis/longhorn/v1beta2"
	"github.com/rancher/lasso/pkg/controller"
	"github.com/rancher/wrangler/v3/pkg/generic"
	"github.com/rancher/wrangler/v3/pkg/schemes"
	"github.com/rancher/wrangler/v3/pkg/start"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	kubevirtv1 "kubevirt.io/api/core/v1"

	healthv1 "github.com/harvester/resource-health-controller/pkg/apis/health.harvesterhci.io/v1beta1"
	"github.com/harvester/resource-health-controller/pkg/crd"
	ctlhealth "github.com/harvester/resource-health-controller/pkg/generated/controllers/health.harvesterhci.io"
)

var (
	localSchemeBuilder = runtime.SchemeBuilder{
		healthv1.AddToScheme,
		harvesterv1.AddToScheme,
		kubevirtv1.AddToScheme,
		longhornv1.AddToScheme,
	}
	AddToScheme = localSchemeBuilder.AddToScheme
	Scheme      = runtime.NewScheme()
)

func init() {
	utilruntime.Must(AddToScheme(Scheme))
	utilruntime.Must(schemes.AddToScheme(Scheme))
}

type RegisterFunc func(context.Context, *Management) error

type Management struct {
	ctx context.Context

	ControllerFactory controller.SharedControllerFactory

	// HealthFactory serves the ComponentHealth / HealthSummary CRDs owned by this addon.
	HealthFactory *ctlhealth.Factory

	// The factories below come from github.com/harvester/harvester and serve the
	// resources whose health is being checked.
	HarvesterFactory *ctlharvester.Factory
	CoreFactory      *ctlcore.Factory
	KubeVirtFactory  *ctlkubevirt.Factory
	LonghornFactory  *ctllonghorn.Factory

	ClientSet *kubernetes.Clientset

	// MinSeverity hides the checks below this severity, empty shows all of them.
	MinSeverity healthv1.Severity

	starters []start.Starter
}

func (s *Management) Start(threadiness int) error {
	return start.All(s.ctx, threadiness, s.starters...)
}

func (s *Management) Register(ctx context.Context, config *rest.Config, registerFuncList []RegisterFunc) error {
	if err := crd.Create(ctx, config); err != nil {
		return err
	}

	for _, f := range registerFuncList {
		if err := f(ctx, s); err != nil {
			return err
		}
	}

	return nil
}

func SetupManagement(ctx context.Context, restConfig *rest.Config) (*Management, error) {
	factory, err := controller.NewSharedControllerFactoryFromConfig(restConfig, Scheme)
	if err != nil {
		return nil, err
	}

	opts := &generic.FactoryOptions{
		SharedControllerFactory: factory,
	}

	management := &Management{
		ctx:               ctx,
		ControllerFactory: factory,
	}

	health, err := ctlhealth.NewFactoryFromConfigWithOptions(restConfig, opts)
	if err != nil {
		return nil, err
	}
	management.HealthFactory = health
	management.starters = append(management.starters, health)

	harvester, err := ctlharvester.NewFactoryFromConfigWithOptions(restConfig, opts)
	if err != nil {
		return nil, err
	}
	management.HarvesterFactory = harvester
	management.starters = append(management.starters, harvester)

	core, err := ctlcore.NewFactoryFromConfigWithOptions(restConfig, opts)
	if err != nil {
		return nil, err
	}
	management.CoreFactory = core
	management.starters = append(management.starters, core)

	kubevirt, err := ctlkubevirt.NewFactoryFromConfigWithOptions(restConfig, opts)
	if err != nil {
		return nil, err
	}
	management.KubeVirtFactory = kubevirt
	management.starters = append(management.starters, kubevirt)

	longhorn, err := ctllonghorn.NewFactoryFromConfigWithOptions(restConfig, opts)
	if err != nil {
		return nil, err
	}
	management.LonghornFactory = longhorn
	management.starters = append(management.starters, longhorn)

	management.ClientSet, err = kubernetes.NewForConfig(restConfig)
	if err != nil {
		return nil, err
	}

	return management, nil
}
