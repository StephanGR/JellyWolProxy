package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
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

func loadConfig(configPath string) (*Config, error) {
	var config Config
	configFile, err := os.ReadFile(configPath)
	if err != nil {
		return nil, err
	}
	err = json.Unmarshal(configFile, &config)
	if err != nil {
		return nil, err
	}
	return &config, nil
}

func sendJellyfinMessagesForSessionsWithPlayingQueue(logger *logrus.Logger, jellyfinUrl, apiKey string) {
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

	// Envoyer un message à chaque session avec une NowPlayingQueue non vide
	for _, session := range sessions {
		if queue, ok := session["NowPlayingQueue"].([]interface{}); ok && len(queue) > 0 {
			if sessionId, ok := session["Id"].(string); ok {
				sendJellyfinMessage(logger, jellyfinUrl, apiKey, sessionId)
			}
		}
	}
}

func sendJellyfinMessage(logger *logrus.Logger, jellyfinUrl, apiKey, sessionId string) {
	messageUrl := fmt.Sprintf("%s/Sessions/%s/message", jellyfinUrl, sessionId)
	payload := strings.NewReader(`{
		"Header": "Information",
		"Text": "Le serveur démarre...\nVeuillez patienter",
		"TimeoutMs": 2000
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

func initConfig() *Config {
	viper.SetConfigName("config") // Nom du fichier de configuration (sans extension)
	viper.SetConfigType("json")   // Par exemple "json", "yaml"
	viper.AddConfigPath(".")      // Par exemple, le chemin vers votre répertoire de configuration

	viper.AutomaticEnv() // Surcharge la configuration avec des variables d'environnement

	if err := viper.ReadInConfig(); err != nil {
		log.Fatalf("Error reading config file, %s", err)
	}

	var config Config
	err := viper.Unmarshal(&config)
	if err != nil {
		log.Fatalf("Unable to decode into struct, %v", err)
	}

	return &config
}

func main() {
	logger := initLogger()

	configPath := flag.String("config", "config.json", "path to config file")
	flag.Parse()

	serverState := &ServerState{}
	config, err := loadConfig(*configPath)
	if err != nil {
		logger.Fatal("Error loading config file: ", err)
	}

	logger.Info("Configuration successfully loaded")

	mux := http.NewServeMux()

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		handler(logger, w, r, *config, serverState)
	})

	mux.HandleFunc("/ping", PingHandler)

	loggedMux := requestLoggerMiddleware(logger, mux)

	logger.Info("Starting app..")
	logger.Fatal(http.ListenAndServe(":3881", loggedMux))
}
