package server

import (
	"github.com/StephanGR/JellyWolProxy/internal/config"
	"github.com/StephanGR/JellyWolProxy/internal/jellyfin"
	"github.com/StephanGR/JellyWolProxy/internal/util"
	"github.com/sirupsen/logrus"
	"time"
)

func WaitServerOnline(logger *logrus.Logger, serverAddress string, config *config.Config) {
	timeout := time.After(2 * time.Minute)
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if util.IsServerUp(logger, serverAddress) {
				logger.Info("Server is up !")
				jellyfin.SendJellyfinMessagesToAllSessions(logger, config.JellyfinUrl, config.ApiKey, "Information ", "\nLe serveur est démarré !\nBon film !")
				return
			}
		case <-timeout:
			logger.Info("Timeout reached, server did not wake up.")
			return
		}
	}
}
