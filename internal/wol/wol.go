package wol

import (
	"github.com/StephanGR/JellyWolProxy/internal/config"
	"github.com/StephanGR/JellyWolProxy/internal/jellyfin"
	"github.com/StephanGR/JellyWolProxy/internal/server_state"
	"github.com/mdlayher/wol"
	"github.com/sirupsen/logrus"
	"net"
)

func WakeServer(logger *logrus.Logger, macAddress string, broadcastAddress string, config config.Config, serverState *server_state.ServerState) {
	if !serverState.StartWakingUp() {
		logger.Info("There is already a wake up in progress")
		return
	}
	defer serverState.DoneWakingUp()

	jellyfin.SendJellyfinMessagesForSessionsWithPlayingQueue(logger, config.JellyfinUrl, config.ApiKey)

	client, err := wol.NewClient()
	if err != nil {
		logger.Warn("Error when creating WOL client : %v", err)
		return
	}
	defer func(client *wol.Client) {
		err := client.Close()
		if err != nil {
			logger.Warn("Unable to close the WOL client")
		}
	}(client)

	mac, err := net.ParseMAC(macAddress)
	if err != nil {
		logger.Warn("Invalid mac address : %v", err)
		return
	}
	if err := client.Wake(broadcastAddress, mac); err != nil {
		logger.Warn("Error when sending magic packet : %v", err)
	} else {
		logger.Info("Magic packet sent")
	}
}
