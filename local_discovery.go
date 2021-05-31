package LocalDiscovery

import (
	"errors"
	"github.com/golang/protobuf/proto"
	"log"
	"net"
	"os"
	"strconv"
	"time"
)

type LocalAnnouncer struct {
	PeerChannel          chan *PotentialPeer
	AnnounceInterval     time.Duration
	stopChannel          chan bool
	packetChannel        chan []byte
	localAnnounceMessage *LocalAnnounce
	listenerUDP4         *listener
	listenerUDP6         *listener
}

type PotentialPeer struct {
	Addresses []net.IP
	Extra     []string
}

type listener struct {
	port        string
	stopChannel chan bool
}

func NewLocalAnnouncer(port uint32, magic uint64, extra []string, announceTime time.Duration) (*LocalAnnouncer, error) {
	msg, err := newLocalAnnounceMessage(port, magic, extra)

	if err != nil {
		return nil, err
	}

	return &LocalAnnouncer{
		PeerChannel:          make(chan *PotentialPeer, 1),
		AnnounceInterval:     announceTime,
		stopChannel:          make(chan bool, 1),
		packetChannel:        make(chan []byte, 1),
		localAnnounceMessage: msg,
		listenerUDP4: &listener{
			port:        strconv.FormatUint(uint64(port), 10),
			stopChannel: make(chan bool, 1),
		},
		listenerUDP6: &listener{
			port:        strconv.FormatUint(uint64(port), 10),
			stopChannel: make(chan bool, 1),
		},
	}, nil
}

func (lA *LocalAnnouncer) Start() {
	go lA.mainLoop()
}

func (lA *LocalAnnouncer) Stop() {
	lA.stopChannel <- true
}

func (lA *LocalAnnouncer) mainLoop() {
	go lA.listenerUDP4.listenToIPUDP("udp4", lA.packetChannel)
	go lA.listenerUDP6.listenToIPUDP("udp6", lA.packetChannel)

	lA.sendUDPPacket()

	for {
		select {
		case <-time.After(lA.AnnounceInterval):
			lA.sendUDPPacket()

		case <-lA.stopChannel:
			lA.listenerUDP4.stopChannel <- true
			lA.listenerUDP6.stopChannel <- true
			return

		case packet := <-lA.packetChannel:
			newAnnounceMessage := &LocalAnnounce{}
			if err := proto.Unmarshal(packet, newAnnounceMessage); err != nil {
				log.Fatalf("Error in packet: %s\n error: %s \n", string(packet), err.Error())
			}

			if !lA.isThisInterface(newAnnounceMessage.Addresses) {
				newPossiblePeer := &PotentialPeer{}

				for _, address := range newAnnounceMessage.GetAddresses() {
					newPossiblePeer.Addresses = append(newPossiblePeer.Addresses, net.ParseIP(address))
				}

				if newAnnounceMessage.Extra != nil {
					newPossiblePeer.Extra = newAnnounceMessage.Extra
				}

				lA.PeerChannel <- newPossiblePeer

			}
		}
	}

}

func (lA *LocalAnnouncer) isThisInterface(addresses []string) bool {
	for _, addr := range lA.localAnnounceMessage.Addresses {
		for _, addr2 := range addresses {
			if addr == addr2 {
				return true
			}
		}
	}
	return false
}

func (lA *LocalAnnouncer) sendUDPPacket() {
	marshalledLocalAnnounce, err := proto.Marshal(lA.localAnnounceMessage)

	if err != nil {
		log.Fatalf("Error Marshalling error: %s \n", err.Error())
	}

	conn, err := net.Dial("udp4", "255.255.255.255:"+lA.listenerUDP4.port)

	if err != nil {
		log.Fatalf("Error in sending updv4 packet: %s", err.Error())
	} else {
		defer conn.Close()
	}

	conn.Write(marshalledLocalAnnounce)

	//conn2, err := net.Dial("udp6", "[ff02::1]:" + lA.listenerUDP6.port)

	//if err != nil {
	//	log.Fatalf("Error in sending updv6 packet: %s", err.Error())
	//} else {
	//	defer conn2.Close()
	//}

	//conn2.Write(marshalledLocalAnnounce)
}

func (l *listener) listenToIPUDP(protocolVersion string, packetChannel chan []byte) {

	pc, err := net.ListenPacket(protocolVersion, ":"+l.port)
	if err != nil {
		log.Fatalf("Error in ListenPacket %s: %s \n", protocolVersion, err.Error())
		return
	}

	defer pc.Close()

	err = pc.SetReadDeadline(time.Now().Add(time.Second * 3))
	if err != nil {
		log.Fatalf("Error in ListenPacket %s: %s \n", protocolVersion, err.Error())
		return
	}
	buf := make([]byte, 65535)

	for {
		select {
		case <-l.stopChannel:
			return

		default:
			n, _, err := pc.ReadFrom(buf)

			if err != nil && errors.Is(err, os.ErrDeadlineExceeded) {
				break
			} else if err != nil {
				log.Fatalf("Error in ListenPacket %s: %s \n", protocolVersion, err.Error())
				return
			}
			packetChannel <- buf[:n]
		}

	}
}

func newLocalAnnounceMessage(port uint32, magic uint64, extra []string) (*LocalAnnounce, error) {
	addressesAsString, err := getLocalAddresses()

	if err != nil {
		return nil, err
	}

	return &LocalAnnounce{
		Magic:     magic,
		Port:      port,
		Addresses: addressesAsString,
		Extra:     extra,
	}, nil
}

func getLocalAddresses() ([]string, error) {
	var addresses []string

	interfaces, err := net.Interfaces()
	if err != nil {
		return nil, err
	}
	for _, i := range interfaces {
		address, err := i.Addrs()
		if err != nil {
			return nil, err
		}
		for _, addr := range address {
			var ip net.IP
			switch v := addr.(type) {
			case *net.IPNet:
				ip = v.IP
				break
			case *net.IPAddr:
				ip = v.IP
				break
			}

			if ipAsString := ip.String(); ipAsString != "<nil>" && isNonLocalIP(ipAsString) {
				addresses = append(addresses, ipAsString)
			}
		}
	}

	return addresses, nil
}

func isNonLocalIP(address string) bool {
	return address != "127.0.0.1" && address != "::1"
}
