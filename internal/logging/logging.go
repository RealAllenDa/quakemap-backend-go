package logging

import (
	"os"
	"strings"

	"github.com/sirupsen/logrus"
	prefixed "github.com/x-cray/logrus-prefixed-formatter"
)

func Configure() {
	forceColors, disableColors := colorPolicy(os.Getenv)

	stdFormatter := &prefixed.TextFormatter{
		FullTimestamp:   true,
		TimestampFormat: "2006-01-02.15:04:05.000000",
		ForceFormatting: true,
		ForceColors:     forceColors,
		DisableColors:   disableColors,
	}
	logrus.SetFormatter(stdFormatter)
	logrus.SetOutput(os.Stdout)

	levelName := strings.TrimSpace(os.Getenv("LOG_LEVEL"))
	if levelName == "" {
		levelName = "info"
	}
	level, err := logrus.ParseLevel(levelName)
	if err != nil {
		logrus.WithField("value", levelName).Warn("invalid LOG_LEVEL; using info")
		level = logrus.InfoLevel
	}
	logrus.SetLevel(level)
}

func colorPolicy(getenv func(string) string) (force, disable bool) {
	if getenv("NO_COLOR") != "" {
		return false, true
	}
	switch strings.ToLower(strings.TrimSpace(getenv("LOG_COLORS"))) {
	case "0", "false", "no", "off":
		return false, true
	default:
		return true, false
	}
}
