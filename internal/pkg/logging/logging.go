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

// Implementation of Log for the audit/request log
type logger struct {
	logger  *logrus.Entry
	logFile *os.File
}

// The one singleton logger
var g_logger logger

func Logger() *logrus.Entry {
	return g_logger.logger
}

func init() {
	// Viper defaults
	viper.SetDefault("logging.location", "stderr")
	viper.SetDefault("logging.format", "text")
	viper.SetDefault("logging.level", "info")

	// The app instantiation ID
	id := uuid.New().String()

	g_logger.logger = logrus.WithFields(logrus.Fields{
		"pid":      os.Getpid(),
		"exe":      path.Base(os.Args[0]),
		"instance": id,
	})
}

func Configure(cfg *viper.Viper) error {
	// Configure system log location
	switch loc := cfg.GetString("location"); loc {
	case "stdout":
		logrus.SetOutput(os.Stdout)
	case "stderr":
		logrus.SetOutput(os.Stderr)
	default:
		file, err := os.OpenFile(loc, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
		if err == nil {
			g_logger.logger.Debugf("Switching system log to %s", loc)
			logrus.SetOutput(file)

			if g_logger.logFile != nil {
				g_logger.logFile.Close()
			}

			g_logger.logFile = file
		} else {
			return err
		}
	}

	// Obey the level setting in the config if not already in debug mode
	if !logrus.IsLevelEnabled(logrus.DebugLevel) {
		level := cfg.GetString("level")
		val, err := logrus.ParseLevel(level)
		if err == nil {
			logrus.SetLevel(val)
		} else {
			return fmt.Errorf("Bad log level: [%s]", level)
		}
	}

	format := cfg.GetString("format")
	if format == "json" {
		logrus.SetFormatter(&logrus.JSONFormatter{})
	}

	// Override the standard system logger
	stdlog.SetOutput(Logger().WriterLevel(logrus.DebugLevel))

	return nil
}
