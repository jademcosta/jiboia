package logger

import (
	"github.com/jademcosta/jiboia/pkg/config"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

const (
	COMPONENT_KEY      = "component"
	EXT_QUEUE_TYPE_KEY = "ext_queue_type"
)

// type Logger *zap.SugaredLogger
//FIXME: use this instead of zap namespace

func New(config *config.Config) *zap.SugaredLogger {

	logLevel, _ := zapcore.ParseLevel(config.Log.Level)
	zapconfig := zap.Config{
		Level:            zap.NewAtomicLevelAt(logLevel),
		Development:      false,
		Encoding:         config.Log.Format,
		EncoderConfig:    encoderConfig(),
		OutputPaths:      []string{"stderr"},
		ErrorOutputPaths: []string{"stderr"},
	}

	logger, err := zapconfig.Build()
	if err != nil {
		panic("Error initializing logger: " + err.Error())
	}

	sugar := logger.Sugar()

	// TODO: Remember to add some signaling so logger can call sync() to flush all logs before exit
	return sugar
}

func encoderConfig() zapcore.EncoderConfig {
	return zapcore.EncoderConfig{
		TimeKey:        "ts",
		LevelKey:       "level",
		NameKey:        "logger",
		CallerKey:      "caller",
		FunctionKey:    zapcore.OmitKey,
		MessageKey:     "msg",
		StacktraceKey:  "stacktrace",
		LineEnding:     zapcore.DefaultLineEnding,
		EncodeLevel:    zapcore.LowercaseLevelEncoder,
		EncodeTime:     zapcore.ISO8601TimeEncoder,
		EncodeDuration: zapcore.SecondsDurationEncoder,
		EncodeCaller:   zapcore.ShortCallerEncoder,
	}
}
