package main

import (
	"fmt"
	"os"

	"github.com/jademcosta/jiboia/pkg/app"
	"github.com/jademcosta/jiboia/pkg/config"
	"github.com/jademcosta/jiboia/pkg/logger"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
)

const version = "0.0.1"

var configPath *string

func main() {
	rootCmd := &cobra.Command{
		Use:   "<command> --config <FILE_PATH>",
		Short: "Starts the app",
		Run:   start,
	}

	setupCommandFlags(rootCmd)

	err := rootCmd.Execute()
	if err != nil {
		panic(fmt.Sprintf("Error on startup: %v", err))
	}
}

func setupCommandFlags(rootCmd *cobra.Command) {
	configPath = rootCmd.Flags().StringP("config", "c", "", "[required]The path for the config file")
	rootCmd.MarkFlagRequired("config")
}

func start(cmd *cobra.Command, args []string) {
	config := initializeConfig()
	l := initializeLogger(*config)

	app.New(config, l).Start()
}

func initializeConfig() *config.Config {

	confData, err := os.ReadFile(*configPath)
	if err != nil {
		panic(fmt.Errorf("error reading config file: %w", err))
	}

	if err != nil {
		panic(err)
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
