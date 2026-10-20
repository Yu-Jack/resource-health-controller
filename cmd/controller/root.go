package main

import (
	"fmt"
	"os"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/harvester/resource-health-controller/pkg/util"
)

var (
	logDebug bool
	logTrace bool

	name             string
	noLeaderElection bool
	threadiness      int
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "resource-health-controller",
	Short: "Resource Health Controller",
	Long: `Resource Health Controller

	The resource health controller watches Harvester resources, reports the
	result of each check to ComponentHealth objects, and aggregates them into
	a single cluster-wide HealthSummary object.
	`,
	Version: AppVersion,
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		logrus.SetOutput(os.Stdout)
		if logDebug {
			logrus.SetLevel(logrus.DebugLevel)
		}
		if logTrace {
			logrus.SetLevel(logrus.TraceLevel)
		}
	},
	Run: func(cmd *cobra.Command, args []string) {
		if err := run(); err != nil {
			fmt.Fprintf(os.Stderr, "%s\n", err.Error())
			os.Exit(1)
		}
	},
}

func init() {
	debug := util.EnvGetBool("RESOURCE_HEALTH_CONTROLLER_DEBUG", false)
	trace := util.EnvGetBool("RESOURCE_HEALTH_CONTROLLER_TRACE", false)

	rootCmd.PersistentFlags().BoolVar(&logDebug, "debug", debug, "set logging level to debug")
	rootCmd.PersistentFlags().BoolVar(&logTrace, "trace", trace, "set logging level to trace")

	rootCmd.Flags().StringVar(&name, "name", os.Getenv("RESOURCE_HEALTH_CONTROLLER_NAME"), "The name of the resource-health-controller instance")
	rootCmd.Flags().BoolVar(&noLeaderElection, "no-leader-election", false, "Run resource-health-controller with leader-election disabled")
	rootCmd.Flags().IntVar(&threadiness, "threadiness", 10, "The number of workers processing the ComponentHealth workqueue")
}

// execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func execute() {
	cobra.CheckErr(rootCmd.Execute())
}
