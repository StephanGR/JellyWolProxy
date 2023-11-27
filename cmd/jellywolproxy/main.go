package main

import (
	"flag"
	"fmt"
	"github.com/StephanGR/JellyWolProxy/internal/config"
	"github.com/StephanGR/JellyWolProxy/internal/handlers"
	"github.com/StephanGR/JellyWolProxy/internal/logger"
	"github.com/StephanGR/JellyWolProxy/internal/middlewares"
	"github.com/StephanGR/JellyWolProxy/internal/server_state"
	"github.com/spf13/viper"
	"net/http"
)

func main() {
	logger := logger.InitLogger("Info")

	configPath := flag.String("config", "config.json", "path to config file")
	port := flag.Int("port", 3881, "port to run the server on")
	flag.Parse()

	viper.SetConfigFile(*configPath)
	viper.AutomaticEnv()

	var cfg config.Config
	if err := viper.ReadInConfig(); err != nil {
		logger.Fatalf("Error reading config file: %v", err)
	}
	if err := viper.Unmarshal(&cfg); err != nil {
		logger.Fatalf("Unable to decode into struct: %v", err)
	}

	serverState := &server_state.ServerState{}

	logger.Info("Configuration successfully loaded")

	mux := http.NewServeMux()

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		handlers.Handler(logger, w, r, cfg, serverState)
	})

	mux.HandleFunc("/ping", handlers.PingHandler)

	loggedMux := middlewares.RequestLoggerMiddleware(logger, mux)

	serverAddress := fmt.Sprintf(":%d", *port)
	logger.Infof("Starting app on port %d..", *port)
	logger.Fatal(http.ListenAndServe(serverAddress, loggedMux))
}
