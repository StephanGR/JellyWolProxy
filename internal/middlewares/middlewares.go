package middlewares

import (
	logger2 "github.com/StephanGR/JellyWolProxy/internal/logger"
	"github.com/sirupsen/logrus"
	"net/http"
)

func RequestLoggerMiddleware(logger *logrus.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		logger2.LogRequest(logger, r)
		next.ServeHTTP(w, r)
	})
}
