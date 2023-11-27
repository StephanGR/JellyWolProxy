package jellyfin

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/sirupsen/logrus"
	"net/http"
)

type JellyfinMessage struct {
	Header    string `json:"Header"`
	Text      string `json:"Text"`
	TimeoutMs int    `json:"TimeoutMs"`
}

func SendJellyfinMessagesToAllSessions(logger *logrus.Logger, jellyfinUrl, apiKey string, header, text string) {
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

	// Envoyer un message à chaque session avec le nouvel en-tête et texte
	for _, session := range sessions {
		if sessionId, ok := session["Id"].(string); ok {
			SendJellyfinMessage(logger, jellyfinUrl, apiKey, sessionId, header, text)
		}
	}
}

func SendJellyfinMessage(logger *logrus.Logger, jellyfinUrl, apiKey, sessionId, header, text string) {
	messageUrl := fmt.Sprintf("%s/Sessions/%s/message", jellyfinUrl, sessionId)
	message := JellyfinMessage{
		Header:    header,
		Text:      text,
		TimeoutMs: 5000,
	}

	messageData, err := json.Marshal(message)
	if err != nil {
		logger.Warn("Error marshalling message data: ", err)
		return
	}

	req, err := http.NewRequest("POST", messageUrl, bytes.NewBuffer(messageData))
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
	logger.Info("Message sent to session ID ", sessionId)
	defer resp.Body.Close()
}
