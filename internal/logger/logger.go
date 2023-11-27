package logger

import (
	"github.com/sirupsen/logrus"
	"net/http"
)

func InitLogger(logLevel string) *logrus.Logger {
	logger := logrus.New()
	logger.SetFormatter(&logrus.TextFormatter{
		FullTimestamp: true,
	})

	level, err := logrus.ParseLevel(logLevel)
	if err != nil {
		// Si le niveau de journalisation n'est pas reconnu, réglez par défaut sur InfoLevel
		logger.Warnf("Invalid log level '%s', falling back to 'info'", logLevel)
		level = logrus.InfoLevel
	}
	logger.SetLevel(level)

	return logger
}

func LogRequest(logger *logrus.Logger, r *http.Request) {
	logger.WithFields(logrus.Fields{
		"client":     r.Header.Get("X-Forwarded-For"), // Replace the value of client by X-Forwarded-For
		"method":     r.Method,
		"user-agent": r.UserAgent(),
		"path":       r.URL.Path,
	}).Info()
}
