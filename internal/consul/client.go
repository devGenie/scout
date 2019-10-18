package consul

import (
	"fmt"
	"math/rand"
	"time"

	consulapi "github.com/hashicorp/consul/api"
)

type ConsulClient struct {
	serverAddr   string
	clientAddr   string
	hostname     string
	consulClient *consulapi.Client
}

func NewConsulClient(serverAddr string, clientAddr string, hostname string) (consulClient *ConsulClient, err error) {
	config := consulapi.DefaultConfig()
	config.Address = serverAddr
	consul, err := consulapi.NewClient(config)

	if err != nil {
		return nil, err
	}
	client := new(ConsulClient)
	client.serverAddr = serverAddr
	client.clientAddr = clientAddr
	client.hostname = hostname
	client.consulClient = consul

	return client, nil
}

func (client *ConsulClient) RegisterHost() error {

	registration := new(consulapi.AgentServiceRegistration)
	registration.ID = client.hostname
	registration.Name = "scout-node"
	registration.Address = client.clientAddr
	registration.Port = 8600
	registration.Check = new(consulapi.AgentServiceCheck)
	registration.Check.HTTP = fmt.Sprintf("http://%s:%v/health", client.clientAddr, 8600)
	registration.Check.Interval = "5s"
	registration.Check.Timeout = "3s"
	client.consulClient.Agent().ServiceRegister(registration)

	return nil
}

func (client *ConsulClient) FetchRandomHost() (node string, err error) {

	agent := client.consulClient.Health()
	services, _, err := agent.ServiceMultipleTags("scout-node", nil, true, nil)
	if err != nil {
		return "", err
	}

	servicesReturned := len(services)

	if servicesReturned > 0 {
		rand.Seed(time.Now().Unix())
		randomIndex := rand.Int() % len(services)
		service := services[randomIndex]
		return service.Service.Address, nil
	}

	return "", nil
}
