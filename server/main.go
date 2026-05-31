package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"

	"github.com/pion/turn/v4"
)

type config struct {
	ListenAddr string
	Port       int
	PublicIP   string
	Realm      string
	Username   string
	Password   string
	MinPort    int
	MaxPort    int
}

func main() {
	cfg, err := loadConfig()
	if err != nil {
		log.Fatal(err)
	}

	server, listenAddress, publicIP, err := startTURNServer(cfg)
	if err != nil {
		log.Fatal(err)
	}
	defer func() {
		if err := server.Close(); err != nil {
			log.Printf("close TURN server: %v", err)
		}
	}()

	log.Printf("TURN UDP listening on %s", listenAddress)
	log.Printf("TURN relay address %s", publicIP.String())
	log.Printf("Web env: TURN_URL=turn:%s:%d TURN_USERNAME=%s TURN_CREDENTIAL=%s",
		publicIP.String(), cfg.Port, cfg.Username, cfg.Password)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	<-ctx.Done()
	log.Print("TURN server stopped")
}

func startTURNServer(cfg config) (*turn.Server, string, net.IP, error) {
	if err := validateConfig(&cfg); err != nil {
		return nil, "", nil, err
	}

	publicIP := net.ParseIP(cfg.PublicIP)
	if publicIP == nil {
		return nil, "", nil, fmt.Errorf("invalid TURN_PUBLIC_IP/public-ip: %q", cfg.PublicIP)
	}

	listenAddress := net.JoinHostPort(cfg.ListenAddr, strconv.Itoa(cfg.Port))
	packetConn, err := net.ListenPacket("udp4", listenAddress)
	if err != nil {
		return nil, "", nil, fmt.Errorf("listen TURN UDP %s: %w", listenAddress, err)
	}

	users := map[string][]byte{
		cfg.Username: turn.GenerateAuthKey(cfg.Username, cfg.Realm, cfg.Password),
	}

	relayGenerator := relayAddressGenerator(cfg, publicIP)
	server, err := turn.NewServer(turn.ServerConfig{
		Realm: cfg.Realm,
		AuthHandler: func(username, realm string, srcAddr net.Addr) ([]byte, bool) {
			key, ok := users[username]
			if !ok {
				log.Printf("auth rejected username=%q from=%s", username, srcAddr.String())
				return nil, false
			}
			return key, true
		},
		PacketConnConfigs: []turn.PacketConnConfig{
			{
				PacketConn:            packetConn,
				RelayAddressGenerator: relayGenerator,
			},
		},
	})
	if err != nil {
		_ = packetConn.Close()
		return nil, "", nil, fmt.Errorf("start TURN server: %w", err)
	}
	return server, packetConn.LocalAddr().String(), publicIP, nil
}

func loadConfig() (config, error) {
	cfg := config{
		ListenAddr: envString("TURN_LISTEN_ADDR", "0.0.0.0"),
		Port:       envInt("TURN_PORT", 3478),
		PublicIP:   envString("TURN_PUBLIC_IP", ""),
		Realm:      envString("TURN_REALM", "lead.remoteassist"),
		Username:   envString("TURN_USERNAME", "lead"),
		Password:   envString("TURN_CREDENTIAL", "leadpass"),
		MinPort:    envInt("TURN_MIN_PORT", 0),
		MaxPort:    envInt("TURN_MAX_PORT", 0),
	}

	flag.StringVar(&cfg.ListenAddr, "listen", cfg.ListenAddr, "UDP listen address")
	flag.IntVar(&cfg.Port, "port", cfg.Port, "UDP TURN listen port")
	flag.StringVar(&cfg.PublicIP, "public-ip", cfg.PublicIP, "Relay IP advertised to WebRTC clients")
	flag.StringVar(&cfg.Realm, "realm", cfg.Realm, "TURN realm")
	flag.StringVar(&cfg.Username, "username", cfg.Username, "TURN username")
	flag.StringVar(&cfg.Password, "password", cfg.Password, "TURN credential")
	flag.IntVar(&cfg.MinPort, "min-port", cfg.MinPort, "Minimum relay port, 0 disables range")
	flag.IntVar(&cfg.MaxPort, "max-port", cfg.MaxPort, "Maximum relay port, 0 disables range")
	flag.Parse()

	if err := validateConfig(&cfg); err != nil {
		return cfg, err
	}
	return cfg, nil
}

func validateConfig(cfg *config) error {
	if cfg.PublicIP == "" {
		ip, err := firstNonLoopbackIPv4()
		if err != nil {
			return fmt.Errorf("TURN_PUBLIC_IP/public-ip is required when auto detection fails: %w", err)
		}
		cfg.PublicIP = ip
	}
	if cfg.Port <= 0 || cfg.Port > 65535 {
		return fmt.Errorf("invalid TURN port %d", cfg.Port)
	}
	if strings.TrimSpace(cfg.Username) == "" || strings.TrimSpace(cfg.Password) == "" {
		return errors.New("TURN username and credential must not be empty")
	}
	if (cfg.MinPort == 0) != (cfg.MaxPort == 0) {
		return errors.New("TURN_MIN_PORT and TURN_MAX_PORT must be set together")
	}
	if cfg.MinPort > 0 && cfg.MinPort > cfg.MaxPort {
		return errors.New("TURN_MIN_PORT must be <= TURN_MAX_PORT")
	}
	if cfg.MinPort < 0 || cfg.MinPort > 65535 || cfg.MaxPort < 0 || cfg.MaxPort > 65535 {
		return errors.New("TURN relay port range must be within 0..65535")
	}
	return nil
}

func relayAddressGenerator(cfg config, publicIP net.IP) turn.RelayAddressGenerator {
	if cfg.MinPort > 0 {
		return &turn.RelayAddressGeneratorPortRange{
			RelayAddress: publicIP,
			Address:      cfg.ListenAddr,
			MinPort:      uint16(cfg.MinPort),
			MaxPort:      uint16(cfg.MaxPort),
		}
	}

	return &turn.RelayAddressGeneratorStatic{
		RelayAddress: publicIP,
		Address:      cfg.ListenAddr,
	}
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
