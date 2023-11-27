package jellyfin

import (
	"encoding/json"
	"fmt"
	"github.com/sirupsen/logrus"
	"net/http"
	"strings"
)

func SendJellyfinMessagesForSessionsWithPlayingQueue(logger *logrus.Logger, jellyfinUrl, apiKey string) {
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
			SendJellyfinMessage(logger, jellyfinUrl, apiKey, sessionId)
		}
	}
}

func SendJellyfinMessage(logger *logrus.Logger, jellyfinUrl, apiKey, sessionId string) {
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
