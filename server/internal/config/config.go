package config

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
)

type Config struct {
	TURN TURNConfig
	HTTP HTTPConfig
	Auth AuthConfig
}

type TURNConfig struct {
	ListenAddr     string
	Port           int
	PublicIP       string
	Realm          string
	Username       string
	Password       string
	MinPort        int
	MaxPort        int
	URL            string
	ICEServersJSON string
	SFUURL         string
}

type HTTPConfig struct {
	Addr string
	Port int
}

type AuthConfig struct {
	DBPath        string
	AdminUsername string
	AdminPassword string
}

type ICEServer struct {
	URLs       any    `json:"urls"`
	Username   string `json:"username,omitempty"`
	Credential string `json:"credential,omitempty"`
}

func Load() (Config, error) {
	cfg := Config{
		TURN: TURNConfig{
			ListenAddr:     envString("TURN_LISTEN_ADDR", "0.0.0.0"),
			Port:           envInt("TURN_PORT", 3478),
			PublicIP:       envString("TURN_PUBLIC_IP", ""),
			Realm:          envString("TURN_REALM", "lead.remoteassist"),
			Username:       envString("TURN_USERNAME", "lead"),
			Password:       envString("TURN_CREDENTIAL", "leadpass"),
			MinPort:        envInt("TURN_MIN_PORT", 0),
			MaxPort:        envInt("TURN_MAX_PORT", 0),
			URL:            envString("TURN_URL", ""),
			ICEServersJSON: envString("ICE_SERVERS_JSON", ""),
			SFUURL:         envString("SFU_URL", ""),
		},
		HTTP: HTTPConfig{
			Addr: envString("HTTP_ADDR", "0.0.0.0"),
			Port: envInt("PORT", 8787),
		},
		Auth: AuthConfig{
			DBPath:        envString("AUTH_DB_PATH", "data/auth.db"),
			AdminUsername: envString("ADMIN_USERNAME", "admin"),
			AdminPassword: envString("ADMIN_PASSWORD", "admin"),
		},
	}

	flag.StringVar(&cfg.TURN.ListenAddr, "listen", cfg.TURN.ListenAddr, "UDP listen address")
	flag.IntVar(&cfg.TURN.Port, "port", cfg.TURN.Port, "UDP TURN listen port")
	flag.StringVar(&cfg.TURN.PublicIP, "public-ip", cfg.TURN.PublicIP, "Relay IP advertised to WebRTC clients")
	flag.StringVar(&cfg.TURN.Realm, "realm", cfg.TURN.Realm, "TURN realm")
	flag.StringVar(&cfg.TURN.Username, "username", cfg.TURN.Username, "TURN username")
	flag.StringVar(&cfg.TURN.Password, "password", cfg.TURN.Password, "TURN credential")
	flag.IntVar(&cfg.TURN.MinPort, "min-port", cfg.TURN.MinPort, "Minimum relay port, 0 disables range")
	flag.IntVar(&cfg.TURN.MaxPort, "max-port", cfg.TURN.MaxPort, "Maximum relay port, 0 disables range")
	flag.StringVar(&cfg.HTTP.Addr, "http-listen", cfg.HTTP.Addr, "HTTP listen address for static web and signaling")
	flag.IntVar(&cfg.HTTP.Port, "http-port", cfg.HTTP.Port, "HTTP listen port for static web and signaling")
	flag.StringVar(&cfg.TURN.SFUURL, "sfu-url", cfg.TURN.SFUURL, "Optional SFU endpoint advertised to clients")
	flag.Parse()

	if err := Validate(&cfg); err != nil {
		return cfg, err
	}
	if err := ValidateHTTP(cfg.HTTP); err != nil {
		return cfg, err
	}
	if err := ValidateAuth(cfg.Auth); err != nil {
		return cfg, err
	}
	return cfg, nil
}

func Validate(cfg *Config) error {
	if cfg.TURN.PublicIP == "" {
		ip, err := firstNonLoopbackIPv4()
		if err != nil {
			return fmt.Errorf("TURN_PUBLIC_IP/public-ip is required when auto detection fails: %w", err)
		}
		cfg.TURN.PublicIP = ip
	}
	if cfg.TURN.Port <= 0 || cfg.TURN.Port > 65535 {
		return fmt.Errorf("invalid TURN port %d", cfg.TURN.Port)
	}
	if strings.TrimSpace(cfg.TURN.Username) == "" || strings.TrimSpace(cfg.TURN.Password) == "" {
		return errors.New("TURN username and credential must not be empty")
	}
	if (cfg.TURN.MinPort == 0) != (cfg.TURN.MaxPort == 0) {
		return errors.New("TURN_MIN_PORT and TURN_MAX_PORT must be set together")
	}
	if cfg.TURN.MinPort > 0 && cfg.TURN.MinPort > cfg.TURN.MaxPort {
		return errors.New("TURN_MIN_PORT must be <= TURN_MAX_PORT")
	}
	if cfg.TURN.MinPort < 0 || cfg.TURN.MinPort > 65535 || cfg.TURN.MaxPort < 0 || cfg.TURN.MaxPort > 65535 {
		return errors.New("TURN relay port range must be within 0..65535")
	}
	return nil
}

func ValidateHTTP(cfg HTTPConfig) error {
	if cfg.Port <= 0 || cfg.Port > 65535 {
		return fmt.Errorf("invalid HTTP port %d", cfg.Port)
	}
	return nil
}

func ValidateAuth(cfg AuthConfig) error {
	if strings.TrimSpace(cfg.AdminUsername) == "" {
		return errors.New("ADMIN_USERNAME must not be empty")
	}
	if strings.TrimSpace(cfg.AdminPassword) == "" {
		return errors.New("ADMIN_PASSWORD must not be empty")
	}
	return nil
}

func BuildICEServers(cfg Config, publicIP string) []ICEServer {
	if servers, ok := parseICEServersJSON(cfg.TURN.ICEServersJSON); ok {
		return servers
	}
	servers := []ICEServer{{URLs: "stun:stun.l.google.com:19302"}}
	turnURL := strings.TrimSpace(cfg.TURN.URL)
	if turnURL == "" {
		turnURL = fmt.Sprintf("turn:%s:%d", publicIP, cfg.TURN.Port)
	}
	if turnURL != "" {
		servers = append(servers, ICEServer{
			URLs:       turnURL,
			Username:   cfg.TURN.Username,
			Credential: cfg.TURN.Password,
		})
	}
	return servers
}

func RelayMode(cfg Config) string {
	if cfg.TURN.SFUURL != "" {
		return "sfu"
	}
	return "turn"
}

func firstNonLoopbackIPv4() (string, error) {
	ifaces, err := net.Interfaces()
	if err != nil {
		return "", err
	}
	for _, iface := range ifaces {
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}
		for _, addr := range addrs {
			var ip net.IP
			switch v := addr.(type) {
			case *net.IPNet:
				ip = v.IP
			case *net.IPAddr:
				ip = v.IP
			}
			if ip4 := ip.To4(); ip4 != nil && !ip4.IsLoopback() {
				return ip4.String(), nil
			}
		}
	}
	return "", errors.New("no non-loopback IPv4 interface found")
}

func parseICEServersJSON(value string) ([]ICEServer, bool) {
	if value == "" {
		return nil, false
	}
	var servers []ICEServer
	if err := json.Unmarshal([]byte(value), &servers); err != nil {
		return nil, false
	}
	return servers, true
}

func envString(name, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(name)); value != "" {
		return value
	}
	return fallback
}

func envInt(name string, fallback int) int {
	value := strings.TrimSpace(os.Getenv(name))
	if value == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}
	return parsed
}
