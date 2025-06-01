package main

import (
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	configprovider "github.com/layer5io/meshkit/config/provider"
	"github.com/layer5io/meshkit/logger"
	"github.com/layer5io/meshsync/internal/config"
	libmeshsync "github.com/layer5io/meshsync/pkg/lib/meshsync"
	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
)

var (
	serviceName  = "meshsync"
	provider     = configprovider.ViperKey
	version      = "Not Set"
	commitsha    = "Not Set"
	pingEndpoint = ":8222/connz"
)

// command line input params
var (
	outputMode        string
	outputFileName    string
	stopAfterDuration time.Duration
)

func main() {
	parseFlags()
	viper.SetDefault("BUILD", version)
	viper.SetDefault("COMMITSHA", commitsha)

	// Initialize Logger instance
	log, errLoggerNew := logger.New(serviceName, logger.Options{
		Format:   logger.SyslogLogFormat,
		LogLevel: int(logrus.InfoLevel),
	})
	if errLoggerNew != nil {
		fmt.Println(errLoggerNew)
		os.Exit(1)
	}

	if err := libmeshsync.Run(
		log,
		libmeshsync.WithOutputMode(outputMode),
		libmeshsync.WithOutputFileName(outputFileName),
		libmeshsync.WithStopAfterDuration(stopAfterDuration),
		libmeshsync.WithVersion(version),
		libmeshsync.WithPingEndpoint(pingEndpoint),
		libmeshsync.WithMeshkitConfigProvider(provider),
	); err != nil {
		log.Error(err)
		os.Exit(1)
	}
}

func parseFlags() {
	notUsedBrokerURL := ""
	flag.StringVar(
		&notUsedBrokerURL,
		"broker-url",
		"",
		"Broker URL (note: primarily configured via BROKER_URL env var; this flag is for compatibility and its value is ignored).",
	)
	flag.StringVar(
		&outputMode,
		"output",
		config.OutputModeNats,
		fmt.Sprintf("output mode: \"%s\" or \"%s\"", config.OutputModeNats, config.OutputModeFile),
	)
	flag.StringVar(
		&outputFileName,
		"outputFile",
		"",
		"output file where to put the meshsync events (cluster snapshot), only applicable for file output mode (default \"./meshery-cluster-snapshot-YYYYMMDD-00.yaml\")",
	)
	flag.StringVar(
		&config.OutputNamespace,
		"outputNamespace",
		"",
		"k8s namespace for which limit the output, f.e. \"default\", applicable for both nats and file output mode",
	)
	var outputResourcesString string
	flag.StringVar(
		&outputResourcesString,
		"outputResources",
		"",
		"k8s resources for which limit the output, coma separated case insensitive list of k8s resources, f.e. \"pod,deployment,service\", applicable for both nats and file output mode",
	)
	flag.DurationVar(
		&stopAfterDuration,
		"stopAfter",
		-1,
		"stop meshsync execution after specified duration, excepts value which is parsable by time.ParseDuration,  f.e. 8s",
	)

	// Parse the command=line flags to get the output mode
	flag.Parse()

	config.OutputResourcesSet = make(map[string]bool)
	if outputResourcesString != "" {
		config.OutputOnlySpecifiedResources = true
		outputResourcesList := strings.Split(outputResourcesString, ",")
		for _, item := range outputResourcesList {
			config.OutputResourcesSet[strings.ToLower(item)] = true
		}
	}
}
