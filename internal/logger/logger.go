package logger

import (
	"github.com/sirupsen/logrus"
	"net/http"
)

func InitLogger() *logrus.Logger {
	logger := logrus.New()
	logger.SetFormatter(&logrus.TextFormatter{
		FullTimestamp: true,
	})
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
