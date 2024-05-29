package main

import (
	"flag"
	"fmt"
	"log/slog"
	"os"

	"github.com/jademcosta/jiboia/pkg/app"
	"github.com/jademcosta/jiboia/pkg/config"
	"github.com/jademcosta/jiboia/pkg/logger"
)

const version = "0.0.1" //FIXME: automatize this

func main() {

	configPath := flag.String("config", "", "<command> --config <FILE_PATH>")
	flag.Parse()

	if *configPath == "" {
		panic("no config file path provided. Usage is: <command> --config <FILE_PATH>")
	}
	fmt.Printf("====>config val: %s\n", *configPath)

	config := initializeConfig(*configPath)
	l := initializeLogger(*config)

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
