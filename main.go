package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/mdlayher/wol"
	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
)

type Config struct {
	JellyfinUrl      string   `mapstructure:"jellyfinUrl"`
	ApiKey           string   `mapstructure:"apiKey"`
	MacAddress       string   `mapstructure:"macAddress"`
	BroadcastAddress string   `mapstructure:"broadcastAddress"`
	WakeUpPort       int      `mapstructure:"wakeUpPort"`
	WakeUpIp         string   `mapstructure:"wakeUpIp"`
	ForwardIp        string   `mapstructure:"forwardIp"`
	ForwardPort      int      `mapstructure:"forwardPort"`
	WakeUpEndpoints  []string `mapstructure:"wakeUpEndpoints"`
}

type ServerState struct {
	wakingUp bool
	lock     sync.Mutex
}

func initLogger() *logrus.Logger {
	logger := logrus.New()
	logger.SetFormatter(&logrus.TextFormatter{
		FullTimestamp: true,
	})
	return logger
}

func logRequest(logger *logrus.Logger, r *http.Request) {
	logger.WithFields(logrus.Fields{
		"client":     r.Header.Get("X-Forwarded-For"), // Replace the value of client by X-Forwarded-For
		"method":     r.Method,
		"user-agent": r.UserAgent(),
		"path":       r.URL.Path,
	}).Info()
}

func (s *ServerState) IsWakingUp() bool {
	s.lock.Lock()
	defer s.lock.Unlock()
	return s.wakingUp
}

func (s *ServerState) StartWakingUp() bool {
	s.lock.Lock()
	defer s.lock.Unlock()
	if s.wakingUp {
		return false // Already waking up
	}
	s.wakingUp = true
	return true
}

func (s *ServerState) DoneWakingUp() {
	s.lock.Lock()
	defer s.lock.Unlock()
	s.wakingUp = false
}

func sendJellyfinMessagesForSessionsWithPlayingQueue(logger *logrus.Logger, jellyfinUrl, apiKey string) {
	logger.Info("Fetching sessions - Jellyfin API")
	sessionsUrl := fmt.Sprintf("%s/Sessions", jellyfinUrl)
	req, err := http.NewRequest("GET", sessionsUrl, nil)
	if err != nil {
		logger.Warn("Error creating request: ", err)
		return
	}
	req.Header.Set("X-MediaBrowser-Token", apiKey)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		logger.Warn("Error getting sessions from Jellyfin: ", err)
		return
	}
	defer resp.Body.Close()

	var sessions []map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&sessions); err != nil {
		logger.Warn("Error decoding sessions response: ", err)
		return
	}

	// Envoyer un message à chaque session
	for _, session := range sessions {
		logger.Info(session)
		if sessionId, ok := session["Id"].(string); ok {
			sendJellyfinMessage(logger, jellyfinUrl, apiKey, sessionId)
		}
	}
}

func sendJellyfinMessage(logger *logrus.Logger, jellyfinUrl, apiKey, sessionId string) {
	messageUrl := fmt.Sprintf("%s/Sessions/%s/message", jellyfinUrl, sessionId)
	payload := strings.NewReader(`{
		"Header": "Information",
		"Text": "Le serveur démarre...\nVeuillez patienter",
		"TimeoutMs": 10000
	}`)

	req, err := http.NewRequest("POST", messageUrl, payload)
	if err != nil {
		logger.Warn("Error creating request: ", err)
		return
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-MediaBrowser-Token", apiKey)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		logger.Warn("Error sending message to Jellyfin: ", err)
		return
	}
	logger.Info("Message sent !")
	defer resp.Body.Close()
}

func isServerUp(logger *logrus.Logger, address string) bool {
	conn, err := net.Dial("tcp", address)
	if err != nil {
		logger.Warn("Failed to connect:", err)
		return false
	}
	conn.Close()
	return true
}

func wakeServer(logger *logrus.Logger, macAddress string, broadcastAddress string, config Config, serverState *ServerState) {
	if !serverState.StartWakingUp() {
		logger.Info("There is already a wake up in progress")
		return
	}
	defer serverState.DoneWakingUp()

	sendJellyfinMessagesForSessionsWithPlayingQueue(logger, config.JellyfinUrl, config.ApiKey)

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

func handleDomainProxy(w http.ResponseWriter, r *http.Request, config Config) {
	proxy := httputil.NewSingleHostReverseProxy(&url.URL{
		Scheme: "http",
		Host:   fmt.Sprintf("%s:%d", config.ForwardIp, config.ForwardPort),
	})

	r.URL.Host = fmt.Sprintf("%s:%d", config.ForwardIp, config.ForwardPort)
	r.URL.Scheme = "http"
	r.Host = fmt.Sprintf("%s:%d", config.ForwardIp, config.ForwardPort)
	proxy.ServeHTTP(w, r)
}

func handler(logger *logrus.Logger, w http.ResponseWriter, r *http.Request, config Config, serverState *ServerState) {
	if shouldWakeServer(r.URL.Path, config.WakeUpEndpoints) {
		serverAddress := fmt.Sprintf("%s:%d", config.WakeUpIp, config.WakeUpPort)
		if !isServerUp(logger, serverAddress) {
			logger.Info("Server is offline, trying to wake up using Wake On Lan")
			wakeServer(logger, config.MacAddress, config.BroadcastAddress, config, serverState)

			waitServerOnline(logger, serverAddress, w)
			return
		} else {
			handleDomainProxy(w, r, config)
		}
	} else {
		handleDomainProxy(w, r, config)
	}
}

func waitServerOnline(logger *logrus.Logger, serverAddress string, w http.ResponseWriter) {
	timeout := time.After(2 * time.Minute)
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if isServerUp(logger, serverAddress) {
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

func PingHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
}

func requestLoggerMiddleware(logger *logrus.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		logRequest(logger, r)
		next.ServeHTTP(w, r)
	})
}

func shouldWakeServer(endpoint string, wakeUpEndpoints []string) bool {
	for _, pattern := range wakeUpEndpoints {
		if pattern == endpoint || matchesPattern(endpoint, pattern) {
			return true
		}
	}
	return false
}

func matchesPattern(endpoint, pattern string) bool {
	splitPattern := strings.Split(pattern, "*")
	if len(splitPattern) != 2 {
		return false // ou retourner une erreur si le motif n'est pas valide
	}

	prefix, suffix := splitPattern[0], splitPattern[1]
	return strings.HasPrefix(endpoint, prefix) && strings.HasSuffix(endpoint, suffix)
}

func main() {
	logger := initLogger()

	configPath := flag.String("config", "config.json", "path to config file")
	port := flag.Int("port", 3881, "port to run the server on")
	flag.Parse()

	viper.SetConfigFile(*configPath)
	viper.AutomaticEnv()

	var config Config
	if err := viper.ReadInConfig(); err != nil {
		logger.Fatalf("Error reading config file: %v", err)
	}
	if err := viper.Unmarshal(&config); err != nil {
		logger.Fatalf("Unable to decode into struct: %v", err)
	}

	serverState := &ServerState{}

	logger.Info("Configuration successfully loaded")

	mux := http.NewServeMux()

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		handler(logger, w, r, config, serverState)
	})

	mux.HandleFunc("/ping", PingHandler)

	loggedMux := requestLoggerMiddleware(logger, mux)

	serverAddress := fmt.Sprintf(":%d", *port)
	logger.Infof("Starting app on port %d..", *port)
	logger.Fatal(http.ListenAndServe(serverAddress, loggedMux))
}
