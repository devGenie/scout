package scoutcore

import (
	"fmt"
	"log"
	"net"
	"os"
	"scout/scoutcore"
	scout "scout/scoutcore"
	"strconv"
	"sync"
	"time"

	"github.com/hashicorp/memberlist"
	"github.com/hashicorp/raft"
	"github.com/hashicorp/serf/serf"
	"github.com/xgfone/netaddr"
)

type RaftNode struct {
	// Hold configuration for the scout raft node
	raft          *raft.Raft
	hostname      string
	ipaddress     string
	network       string
	raftPort      int
	bindPort      int
	broadcastPort int
	role          string
	store         *RaftStore
	fsm           *FSM
	udpConn       *net.UDPConn
	serfEvents    chan serf.Event
	serfScout     *serf.Serf
	waiter        sync.WaitGroup
	couchbaseNode *scout.CouchbaseNode
}

func NewNode(raftPort int, bindPort int, datacenter string, couchbaseNode *scout.CouchbaseNode) *RaftNode {
	hostname := scoutcore.HostName()
	ipaddr := scoutcore.IPAddr()
	node := &RaftNode{
		hostname:      hostname,
		ipaddress:     ipaddr,
		store:         new(RaftStore),
		fsm:           new(FSM),
		raftPort:      raftPort,
		bindPort:      bindPort,
		broadcastPort: 1300,
		network:       datacenter,
		serfEvents:    make(chan serf.Event, 16),
		couchbaseNode: couchbaseNode,
	}
	return node
}

func (node *RaftNode) Run() error {
	memberlistConfig := memberlist.DefaultLANConfig()
	memberlistConfig.BindAddr = node.ipaddress
	memberlistConfig.BindPort = node.bindPort
	memberlistConfig.LogOutput = os.Stdout

	serfConfig := serf.DefaultConfig()
	serfConfig.NodeName = node.hostname
	serfConfig.EventCh = node.serfEvents
	serfConfig.MemberlistConfig = memberlistConfig
	serfConfig.LogOutput = os.Stdout

	serfScout, err := serf.Create(serfConfig)
	if err != nil {
		return err
	}

	node.serfScout = serfScout
	node.store.raftAddr = fmt.Sprintf("%s:%d", node.ipaddress, node.raftPort)

	if err != nil {
		return err
	}

	node.store.dbPath = "/etc/"

	err = node.store.Init()
	if err != nil {
		return err
	}

	err = node.store.BootstrapStore()
	if err != nil {
		return err
	}

	node.waiter.Add(2)
	go node.listenUDP()
	go node.ticker()
	node.waiter.Wait()
	return nil
}

func (node *RaftNode) joinCluster(remote string) {
	fmt.Println("Joining ", remote)
	nodes := []string{remote}
	_, err := node.serfScout.Join(nodes, false)

	if err != nil {
		fmt.Println("error joing peer")
		log.Fatal(err)
		return
	}

	fmt.Println("Adding node to ", remote)
	err = node.couchbaseNode.AddNode(remote + ":8091")

	if err != nil {
		log.Println("Error adding this node to cluster")
		log.Fatalln(err)
		return
	}

	log.Println("Node successfully added to cluster")

}
func (node *RaftNode) broadcast() {

	netIP := netaddr.MustNewIPNetwork(node.network)
	bcast := netIP.Broadcast()

	broadcastUDPAddr, err := net.ResolveUDPAddr("udp", fmt.Sprintf("%s:%d", bcast.String(), node.broadcastPort))

	packet := new(scoutcore.Packet)
	packet.Header = scoutcore.HELLO

	encodedPacket, err := scoutcore.Encode(packet)
	if err != nil {
		log.Fatal("error decoding packet ", err)
	}
	log.Println("Broadcast address ", broadcastUDPAddr)
	n, err := node.udpConn.WriteToUDP(encodedPacket, broadcastUDPAddr)

	if err != nil {
		fmt.Println("error sending Broadcast message")
		log.Fatalln(err)
	}
	fmt.Println(n)
}

func (node *RaftNode) listenUDP() {
	defer node.waiter.Done()
	udpAddr, err := net.ResolveUDPAddr("udp", fmt.Sprintf("%s:%d", "0.0.0.0", node.broadcastPort))
	if err != nil {
		fmt.Println("error parsing broadcast IP address")
		log.Fatal(err)
	}
	udpConn, err := net.ListenUDP("udp", udpAddr)
	if err != nil {
		fmt.Println("error dialing UDP address")
	}

	node.udpConn = udpConn
	buffer := make([]byte, 2000)

	log.Println("listening address ", udpAddr.String())
	node.broadcast()

	for {
		length, addr, err := node.udpConn.ReadFromUDP(buffer)

		if err != nil || length == 0 {
			log.Fatal("error reading from UDP ", err)
			continue
		}

		packet := new(scout.Packet)
		err = scout.Decode(packet, buffer[:length])

		if err != nil {
			log.Fatalf("error decoding packet from %s \t Error: %s \n", addr.String(), err)
		}

		switch packet.Header {
		case scout.HELLO:
			remoteAddr := addr.IP.String()
			if remoteAddr != node.ipaddress {
				node.processHandshake(addr)
			}
		case scout.HELLOREPLY:
			remoteIP := string(packet.Payload)
			fmt.Println("eecieved broadcast reply from", remoteIP)
			node.joinCluster(remoteIP)
		default:
			fmt.Println("expected headers not found")
		}
	}
}

func (node *RaftNode) IsLeader() bool {
	leader := node.store.raft.VerifyLeader()

	if err := leader.Error(); err != nil {
		return false
	}
	return true
}

func (node *RaftNode) processHandshake(addr *net.UDPAddr) {
	fmt.Println("processing handshake from ", addr.String())

	isleader := node.IsLeader()

	if isleader {
		packet := new(scout.Packet)
		packet.Header = scout.HELLOREPLY
		packet.Payload = []byte(node.ipaddress)

		encodedPacket, err := scout.Encode(packet)

		if err != nil {
			fmt.Println(err)
		}

		node.udpConn.WriteToUDP(encodedPacket, addr)
		return
	}

	log.Printf("I am not the leader so I will ignore %s's handshake", addr.String())
}

func (node *RaftNode) ticker() {
	defer node.waiter.Done()
	ticker := time.NewTicker(3 * time.Second)
	for {
		select {
		case <-ticker.C:
			isleader := node.IsLeader()

			fmt.Printf("Showing peers known by %s: \n", node.ipaddress)

			if isleader {
				log.Println("node is a leader")
			} else {
				log.Println("node is a follower")
			}

			futureConfig := node.store.raft.GetConfiguration()

			if err := futureConfig.Error(); err != nil {
				log.Fatalf("error getting config: %s", err)
			}

			config := futureConfig.Configuration()

			for _, server := range config.Servers {
				fmt.Println(server.Address)
			}
		case voterEvent := <-node.serfEvents:
			fmt.Println("processing voter event ", voterEvent)
			isleader := node.IsLeader()

			if memberEvent, ok := voterEvent.(serf.MemberEvent); ok {
				for _, member := range memberEvent.Members {
					changedPeer := member.Addr.String() + ":" + strconv.Itoa(int(member.Port+1))

					if memberEvent.EventType() == serf.EventMemberJoin {
						if isleader == true {
							future := node.store.raft.AddVoter(raft.ServerID(changedPeer), raft.ServerAddress(changedPeer), 0, 0)

							if err := future.Error(); err != nil {
								log.Fatalf("error adding voter: %s", err)
							}
						}
					} else if memberEvent.EventType() == serf.EventMemberLeave || memberEvent.EventType() == serf.EventMemberFailed || memberEvent.EventType() == serf.EventMemberReap {
						if isleader == true {
							future := node.store.raft.RemoveServer(raft.ServerID(changedPeer), 0, 0)

							if err := future.Error(); err != nil {
								log.Fatalf("error removing server %s", err)
							}
						}
					}
				}
			}
		}
	}
}
