package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/jademcosta/jiboia/pkg/app"
	"github.com/jademcosta/jiboia/pkg/config"
	"github.com/jademcosta/jiboia/pkg/logger"
	"go.uber.org/zap"
)

const version = "0.0.1" //FIXME: automatize this

var configPath *string

func main() {

	flag.StringVar(configPath, "config", "", "<command> --config <FILE_PATH>")
	flag.Parse()
	if *configPath == "" {
		panic("no config file path provided. Usage is: <command> --config <FILE_PATH>")
	}

	config := initializeConfig()
	l := initializeLogger(*config)

	app.New(config, l).Start()
}

func initializeConfig() *config.Config {
	confData, err := os.ReadFile(*configPath)
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

func initializeLogger(c config.Config) *zap.SugaredLogger {
	l := logger.New(&c)
	return l
}
