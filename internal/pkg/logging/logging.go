package logging

import (
	"fmt"
	"os"
	"path"

	stdlog "log"

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
)

/*
 *  Provides request and diagnostics logging facilities
 */

type logger struct {
	logger  *logrus.Entry
	logFile *os.File
}

// The one singleton logger
var g_logger logger
var g_instanceId string

func Logger() *logrus.Entry {
	return g_logger.logger
}

func init() {
	// Viper defaults
	viper.SetDefault("logging.location", "stderr")
	viper.SetDefault("logging.format", "text")
	viper.SetDefault("logging.level", "info")

	// The app instantiation ID
	g_instanceId = uuid.New().String()

	g_logger.logger = logrus.WithFields(logrus.Fields{
		"pid":      os.Getpid(),
		"exe":      path.Base(os.Args[0]),
		"instance": g_instanceId,
	})
}

func Configure(cfg *viper.Viper) error {
	// Configure system log location
	switch loc := cfg.GetString("logging.location"); loc {
	case "stdout":
		logrus.SetOutput(os.Stdout)
		g_logger.logger = logrus.WithFields(logrus.Fields{})
	case "stderr":
		logrus.SetOutput(os.Stderr)
		g_logger.logger = logrus.WithFields(logrus.Fields{})
	default:
		file, err := os.OpenFile(loc, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
		if err == nil {
			g_logger.logger.Debugf("Switching system log to %s", loc)
			logrus.SetOutput(file)

			if g_logger.logFile != nil {
				g_logger.logFile.Close()
			}

			g_logger.logFile = file

			g_logger.logger = logrus.WithFields(logrus.Fields{
				"pid":      os.Getpid(),
				"exe":      path.Base(os.Args[0]),
				"instance": g_instanceId,
			})

		} else {
			return err
		}
	}

	// Obey the level setting in the config if not already in debug mode
	if !logrus.IsLevelEnabled(logrus.DebugLevel) {
		level := cfg.GetString("logging.level")
		val, err := logrus.ParseLevel(level)
		if err == nil {
			logrus.SetLevel(val)
		} else {
			return fmt.Errorf("Bad log level: [%s]", level)
		}
	}

	format := cfg.GetString("logging.format")
	if format == "json" {
		logrus.SetFormatter(&logrus.JSONFormatter{})
	}

	// Override the standard system logger
	stdlog.SetOutput(Logger().WriterLevel(logrus.DebugLevel))

	return nil
}
