package turnserver

import (
	"fmt"
	"log"
	"net"
	"strconv"

	"github.com/pion/turn/v4"
	"lead-turn-server/internal/config"
)

func Start(cfg config.Config) (*turn.Server, string, net.IP, error) {
	if err := config.Validate(&cfg); err != nil {
		return nil, "", nil, err
	}

	publicIP := net.ParseIP(cfg.TURN.PublicIP)
	if publicIP == nil {
		return nil, "", nil, fmt.Errorf("invalid TURN_PUBLIC_IP/public-ip: %q", cfg.TURN.PublicIP)
	}

	listenAddress := net.JoinHostPort(cfg.TURN.ListenAddr, strconv.Itoa(cfg.TURN.Port))
	packetConn, err := net.ListenPacket("udp4", listenAddress)
	if err != nil {
		return nil, "", nil, fmt.Errorf("listen TURN UDP %s: %w", listenAddress, err)
	}

	users := map[string][]byte{
		cfg.TURN.Username: turn.GenerateAuthKey(cfg.TURN.Username, cfg.TURN.Realm, cfg.TURN.Password),
	}

	server, err := turn.NewServer(turn.ServerConfig{
		Realm: cfg.TURN.Realm,
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
				RelayAddressGenerator: relayAddressGenerator(cfg, publicIP),
			},
		},
	})
	if err != nil {
		_ = packetConn.Close()
		return nil, "", nil, fmt.Errorf("start TURN server: %w", err)
	}
	return server, packetConn.LocalAddr().String(), publicIP, nil
}

func relayAddressGenerator(cfg config.Config, publicIP net.IP) turn.RelayAddressGenerator {
	if cfg.TURN.MinPort > 0 {
		return &turn.RelayAddressGeneratorPortRange{
			RelayAddress: publicIP,
			Address:      cfg.TURN.ListenAddr,
			MinPort:      uint16(cfg.TURN.MinPort),
			MaxPort:      uint16(cfg.TURN.MaxPort),
		}
	}

	return &turn.RelayAddressGeneratorStatic{
		RelayAddress: publicIP,
		Address:      cfg.TURN.ListenAddr,
	}
}
