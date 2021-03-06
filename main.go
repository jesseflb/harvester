//go:generate go run pkg/codegen/cleanup/main.go
//go:generate /bin/rm -rf pkg/generated
//go:generate go run pkg/codegen/main.go
//go:generate /bin/bash scripts/generate-manifest

package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	_ "net/http/pprof"
	"os"

	"github.com/ehazlett/simplelog"
	"github.com/rancher/wrangler/pkg/signals"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"

	"github.com/rancher/harvester/pkg/config"
	"github.com/rancher/harvester/pkg/server"
)

var (
	Version        = "v0.0.0-dev"
	GitCommit      = "HEAD"
	profileAddress = "localhost:6060"
	KubeConfig     string
)

func main() {
	var options config.Options

	app := cli.NewApp()
	app.Name = "rancher-harvester"
	app.Version = fmt.Sprintf("%s (%s)", Version, GitCommit)
	app.Usage = ""
	app.Flags = []cli.Flag{
		cli.StringFlag{
			Name:        "kubeconfig",
			EnvVar:      "KUBECONFIG",
			Usage:       "Kube config for accessing k8s cluster",
			Destination: &KubeConfig,
		},
		cli.BoolFlag{
			Name:        "debug",
			EnvVar:      "HARVESTER_DEBUG",
			Usage:       "Enable debug logs",
			Destination: &options.Debug,
		},
		cli.BoolFlag{
			Name:        "trace",
			EnvVar:      "HARVESTER_TRACE",
			Usage:       "Enable trace logs",
			Destination: &options.Trace,
		},
		cli.IntFlag{
			Name:        "threadiness",
			EnvVar:      "THREADINESS",
			Usage:       "Specify controller threads",
			Value:       10,
			Destination: &options.Threadiness,
		},
		cli.IntFlag{
			Name:        "http-port",
			EnvVar:      "HARVESTER_SERVER_HTTP_PORT",
			Usage:       "HTTP listen port",
			Value:       8080,
			Destination: &options.HTTPListenPort,
		},
		cli.IntFlag{
			Name:        "https-port",
			EnvVar:      "HARVESTER_SERVER_HTTPS_PORT",
			Usage:       "HTTPS listen port",
			Value:       8443,
			Destination: &options.HTTPSListenPort,
		},
		cli.StringFlag{
			Name:        "namespace",
			EnvVar:      "NAMESPACE",
			Destination: &options.Namespace,
			Usage:       "The default namespace to store management resources",
			Required:    true,
		},
		cli.StringFlag{
			Name:        "image-storage-endpoint",
			Usage:       "S3 compatible storage endpoint(format: http://example.com:9000). It should be accessible across the cluster",
			EnvVar:      "IMAGE_STORAGE_ENDPOINT",
			Destination: &options.ImageStorageEndpoint,
			Required:    true,
		},
		cli.StringFlag{
			Name:        "image-storage-access-key",
			Usage:       "Image storage access key",
			EnvVar:      "IMAGE_STORAGE_ACCESS_KEY",
			Destination: &options.ImageStorageAccessKey,
			Required:    true,
		},
		cli.StringFlag{
			Name:        "image-storage-secret-key",
			Usage:       "Image storage secret key",
			EnvVar:      "IMAGE_STORAGE_SECRET_KEY",
			Destination: &options.ImageStorageSecretKey,
			Required:    true,
		},
		cli.BoolFlag{
			Name:        "skip-authentication",
			EnvVar:      "SKIP_AUTHENTICATION",
			Usage:       "Define whether to skip auth login or not, default to false",
			Destination: &options.SkipAuthentication,
		},
		cli.StringFlag{
			Name:   "authentication-mode",
			EnvVar: "HARVESTER_AUTHENTICATION_MODE",
			Usage:  "Define authentication mode, kubernetesCredentials and localUser are supported, could config more than one mode, separated by comma",
		},
		cli.StringFlag{
			Name:        "profile-listen-address",
			Value:       "127.0.0.1:6060",
			Usage:       "Address to listen on for profiling",
			Destination: &profileAddress,
		},
	}
	app.Action = func(c *cli.Context) error {
		// enable profiler
		if profileAddress != "" {
			go func() {
				log.Println(http.ListenAndServe(profileAddress, nil))
			}()
		}
		initLogs(c, options)
		return run(c, options)
	}

	if err := app.Run(os.Args); err != nil {
		logrus.Fatal(err)
	}
}

func initLogs(c *cli.Context, options config.Options) {
	switch c.String("log-format") {
	case "simple":
		logrus.SetFormatter(&simplelog.StandardFormatter{})
	case "text":
		logrus.SetFormatter(&logrus.TextFormatter{})
	case "json":
		logrus.SetFormatter(&logrus.JSONFormatter{})
	}
	logrus.SetOutput(os.Stdout)
	if options.Debug {
		logrus.SetLevel(logrus.DebugLevel)
		logrus.Debugf("Loglevel set to [%v]", logrus.DebugLevel)
	}
	if options.Trace {
		logrus.SetLevel(logrus.TraceLevel)
		logrus.Tracef("Loglevel set to [%v]", logrus.TraceLevel)
	}
}

func run(c *cli.Context, options config.Options) error {
	logrus.Info("Starting controller")
	ctx := signals.SetupSignalHandler(context.Background())

	kubeConfig, err := server.GetConfig(KubeConfig)
	if err != nil {
		return fmt.Errorf("failed to find kubeconfig: %v", err)
	}

	harv, err := server.New(ctx, kubeConfig, options)
	if err != nil {
		return fmt.Errorf("failed to create harvester server: %v", err)
	}
	return harv.ListenAndServe(nil, options)
}
