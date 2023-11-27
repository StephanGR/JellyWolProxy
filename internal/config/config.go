package config

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
