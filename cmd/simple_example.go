package main

import (
	"LocalDiscovery"
	"log"
	"time"
)

func main() {
	localAnnouncer, err := LocalDiscovery.NewLocalAnnouncer(8080, 9, nil, time.Second)

	if err != nil {
		log.Fatalf("Error creating Announce packet: %s\n", err.Error())
	}

	localAnnouncer.Start()

	for {
		select {
		case potentialPeer := <-localAnnouncer.PeerChannel:
			log.Printf("New peer:")
			for _, addr := range potentialPeer.Addresses {
				log.Printf("ip: %s", addr.String())
			}

			break
		}
	}

	defer localAnnouncer.Stop()
	return
}
