package main

import (
	"flag"
	"fmt"
	"log/slog"
	"os"

	"github.com/jademcosta/jiboia/pkg/app"
	"github.com/jademcosta/jiboia/pkg/config"
	"github.com/jademcosta/jiboia/pkg/logger"
	"go.uber.org/automaxprocs/maxprocs"
)

const version = "0.0.1" //FIXME: automatize this

func main() {

	configPath := flag.String("config", "", "<command> --config <FILE_PATH>")
	flag.Parse()

	if *configPath == "" {
		panic("no config file path provided. Usage is: <command> --config <FILE_PATH>")
	}

	config := initializeConfig(*configPath)
	l := initializeLogger(*config)

	if !config.DisableMaxProcs {
		_, err := maxprocs.Set(maxprocs.Logger(l.Info))
		if err != nil {
			panic(err)
		}
	}

	app.New(config, l).Start()
}

func initializeConfig(configPath string) *config.Config {
	confData, err := os.ReadFile(configPath)
	if err != nil {
		panic(fmt.Errorf("error reading config file: %w", err))
	}

	c, err := config.New(confData)
	if err != nil {
		panic(fmt.Errorf("error initializing/parsing config: %w", err))
	}

	c.Version = version

	return c
}

func initializeLogger(c config.Config) *slog.Logger {
	l := logger.New(&c.Log)
	return l
}
