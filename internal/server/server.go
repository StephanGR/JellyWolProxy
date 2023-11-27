package server

import (
	"fmt"
	"github.com/StephanGR/JellyWolProxy/internal/util"
	"github.com/sirupsen/logrus"
	"net/http"
	"time"
)

func WaitServerOnline(logger *logrus.Logger, serverAddress string, w http.ResponseWriter) {
	timeout := time.After(2 * time.Minute)
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if util.IsServerUp(logger, serverAddress) {
				logger.Info("Server is up !")
				return
			}
		case <-timeout:
			logger.Info("Timeout reached, server did not wake up.")
			fmt.Fprintf(w, "Server is still offline. Please try again later.")
			return
		}
	}
}
