package raft

import (
	"fmt"
	"log"
	"net"
	"os"
	"github.com/devgenie/scout/internal/consul"
	"github.com/devgenie/scout/internal/couchbase"
	"strconv"
	"strings"
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
	voterPort     int
	broadcastPort int
	role          string
	store         *RaftStore
	fsm           *FSM
	udpConn       *net.UDPConn
	serfEvents    chan serf.Event
	serfScout     *serf.Serf
	waiter        sync.WaitGroup
	couchbaseNode *couchbase.CouchbaseNode
	discovery     couchbase.Discovery
}

func NewNode(raftPort int, bindPort int, voterPort int, couchbaseNode *couchbase.CouchbaseNode, discoveryMode couchbase.Discovery) *RaftNode {
	hostname := couchbase.HostName()
	ipaddr := couchbase.IPAddr()
	node := &RaftNode{
		hostname:      hostname,
		ipaddress:     ipaddr,
		store:         new(RaftStore),
		fsm:           new(FSM),
		raftPort:      raftPort,
		voterPort:     voterPort,
		bindPort:      bindPort,
		broadcastPort: 1300,
		//network:       datacenter,
		serfEvents:    make(chan serf.Event, 16),
		couchbaseNode: couchbaseNode,
		discovery:     discoveryMode,
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

	node.store.dbPath = "/tmp/"

	err = node.store.Init()
	if err != nil {
		return err
	}

	err = node.store.BootstrapStore()
	if err != nil {
		return err
	}

	node.waiter.Add(1)

	if node.discovery.Mode == "consul" {
		node.findWithConsul()
	}
	// go node.listenUDP()
	go node.ticker()
	go couchbase.RunWebServer()
	node.waiter.Wait()
	return nil
}

func (node *RaftNode) findWithConsul() error {
	client, err := consul.NewConsulClient(node.discovery.Join, node.ipaddress, node.hostname)

	if err != nil {
		log.Println(err)
		return err
	}

	tries := 0

	for tries < 10 {
		host, err := client.FetchRandomHost()
		if err != nil {
			log.Println(err)
		}

		if len(strings.TrimSpace(host)) != 0 {
			err = node.joinCluster(host)
			if err != nil {
				log.Println("Error joining cluster", err)
			}
			break
		}
		log.Println("Did not find a living node, sleeping for 3 seconds before retrying")
		time.Sleep(3 * time.Second)
		tries++
	}

	log.Println("Failed to find a living node, I will become the leader")
	client.RegisterHost()
	return nil
}

func (node *RaftNode) joinCluster(remote string) error {
	fmt.Println("Joining ", remote)
	nodes := []string{remote}
	_, err := node.serfScout.Join(nodes, false)

	if err != nil {
		fmt.Println("error joing peer")
		return err
	}

	fmt.Println("Adding node to ", remote)
	err = node.couchbaseNode.AddNode(remote + ":8091")

	if err != nil {
		log.Println("Error adding this node to cluster")
		return err
	}

	log.Println("Node successfully added to cluster")
	return nil
}
func (node *RaftNode) broadcast() {

	netIP := netaddr.MustNewIPNetwork(node.network)
	bcast := netIP.Broadcast()

	broadcastUDPAddr, err := net.ResolveUDPAddr("udp", fmt.Sprintf("%s:%d", bcast.String(), node.broadcastPort))

	packet := new(couchbase.Packet)
	packet.Header = couchbase.HELLO

	encodedPacket, err := couchbase.Encode(packet)
	if err != nil {
		log.Fatal("error decoding packet ", err)
	}
	log.Println("Broadcast address ", broadcastUDPAddr)
	_, err = node.udpConn.WriteToUDP(encodedPacket, broadcastUDPAddr)

	if err != nil {
		fmt.Println("error sending Broadcast message")
		log.Fatalln(err)
	}
}

func (node *RaftNode) listenUDP() {
	defer node.waiter.Done()
	fmt.Println(node.broadcastPort)
	udpAddr, err := net.ResolveUDPAddr("udp4", fmt.Sprintf("%s:%d", "0.0.0.0", node.broadcastPort))
	if err != nil {
		fmt.Println("error parsing broadcast IP address")
		log.Fatal(err)
	}
	udpConn, err := net.ListenUDP("udp4", udpAddr)
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

		packet := new(couchbase.Packet)
		err = couchbase.Decode(packet, buffer[:length])

		if err != nil {
			log.Fatalf("error decoding packet from %s \t Error: %s \n", addr.String(), err)
		}

		switch packet.Header {
		case couchbase.HELLO:
			remoteAddr := addr.IP.String()
			if remoteAddr != node.ipaddress {
				node.processHandshake(addr)
			}
		case couchbase.HELLOREPLY:
			remoteIP := string(packet.Payload)
			fmt.Println("recieved broadcast reply from", remoteIP)
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
		packet := new(couchbase.Packet)
		packet.Header = couchbase.HELLOREPLY
		packet.Payload = []byte(node.ipaddress)

		encodedPacket, err := couchbase.Encode(packet)

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
				leaderLastSeen := node.store.raft.LastContact()

				if time.Since(leaderLastSeen) > 10*time.Second {
					err := node.store.Reset()

					if err != nil {
						fmt.Println(err)
					}
				}
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
					changedPeer := member.Addr.String() + ":" + strconv.Itoa(node.raftPort)

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
