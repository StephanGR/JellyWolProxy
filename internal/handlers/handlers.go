package handlers

import (
	"fmt"
	"github.com/StephanGR/JellyWolProxy/internal/config"
	"github.com/StephanGR/JellyWolProxy/internal/server"
	"github.com/StephanGR/JellyWolProxy/internal/server_state"
	"github.com/StephanGR/JellyWolProxy/internal/util"
	"github.com/StephanGR/JellyWolProxy/internal/wol"
	"github.com/sirupsen/logrus"
	"net/http"
	"net/http/httputil"
	"net/url"
)

func handleDomainProxy(w http.ResponseWriter, r *http.Request, config config.Config) {
	proxy := httputil.NewSingleHostReverseProxy(&url.URL{
		Scheme: "http",
		Host:   fmt.Sprintf("%s:%d", config.ForwardIp, config.ForwardPort),
	})

	r.URL.Host = fmt.Sprintf("%s:%d", config.ForwardIp, config.ForwardPort)
	r.URL.Scheme = "http"
	r.Host = fmt.Sprintf("%s:%d", config.ForwardIp, config.ForwardPort)
	proxy.ServeHTTP(w, r)
}

func Handler(logger *logrus.Logger, w http.ResponseWriter, r *http.Request, config config.Config, serverState *server_state.ServerState) {
	logger.Debug("Request received for path: ", r.URL.Path)
	if util.ShouldWakeServer(r.URL.Path, config.WakeUpEndpoints) {
		serverAddress := fmt.Sprintf("%s:%d", config.WakeUpIp, config.WakeUpPort)
		logger.Debug("Wake-up endpoint matched, checking server status...")
		if !util.IsServerUp(logger, serverAddress) {
			logger.Info("Server is offline, trying to wake up using Wake On Lan")
			wol.WakeServer(logger, config.MacAddress, config.BroadcastAddress, config, serverState)

			server.WaitServerOnline(logger, serverAddress, &config)
			return
		} else {
			logger.Info("Server is already online, handling domain proxy...")
			handleDomainProxy(w, r, config)
		}
	} else {
		logger.Info("No wake-up endpoint matched, handling domain proxy...")
		handleDomainProxy(w, r, config)
	}
}

func PingHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
}
