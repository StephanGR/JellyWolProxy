package util

import (
	"github.com/sirupsen/logrus"
	"net"
	"strings"
)

func matchesPattern(endpoint, pattern string) bool {
	splitPattern := strings.Split(pattern, "*")
	if len(splitPattern) != 2 {
		return false // ou retourner une erreur si le motif n'est pas valide
	}

	prefix, suffix := splitPattern[0], splitPattern[1]
	return strings.HasPrefix(endpoint, prefix) && strings.HasSuffix(endpoint, suffix)
}

func IsServerUp(logger *logrus.Logger, address string) bool {
	conn, err := net.Dial("tcp", address)
	if err != nil {
		logger.Warn("Failed to connect:", err)
		return false
	}
	conn.Close()
	return true
}

func ShouldWakeServer(endpoint string, wakeUpEndpoints []string) bool {
	for _, pattern := range wakeUpEndpoints {
		if pattern == endpoint || matchesPattern(endpoint, pattern) {
			return true
		}
	}
	return false
}
