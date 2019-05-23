package scoutcore

import (
	"fmt"
	"log"
)

type CouchbaseNode struct {
	// Hold configurations for ciouchbase node
	Auth     Auth
	Address  string
	Hostname string
	Services string
}

func (node *CouchbaseNode) BoootStrap() error {
	log.Println("Initializing local node")

	log.Println("Setting up services")
	requestBody := make(map[string]string)
	requestBody["services"] = "kv,n1ql,index,fts"
	remoteEndpoint := fmt.Sprintf("http://%s:8091/node/controller/setupServices", node.Address)

	respcode, body, err := SendRequest("POST", remoteEndpoint, requestBody, node.Auth)

	if err != nil || respcode != 200 {
		errMsg := fmt.Sprintf("error setting up services : %s", body)
		return fmt.Errorf(errMsg)
	}

	requestBody = make(map[string]string)
	requestBody["password"] = node.Auth.Password
	requestBody["username"] = node.Auth.Username
	requestBody["port"] = "SAME"
	remoteEndpoint = fmt.Sprintf("http://%s:8091/settings/web", node.Address)

	respcode, body, err = SendRequest("POST", remoteEndpoint, requestBody, node.Auth)

	if err != nil || respcode != 200 {
		errMsg := fmt.Sprintf("error initializing node : %s", body)
		return fmt.Errorf(errMsg)
	}

	fmt.Println("1: initializing local node node")
	requestBody = make(map[string]string)
	requestBody["data_path"] = "/opt/couchbase/var/lib/couchbase/data"
	requestBody["index_path"] = "/opt/couchbase/var/lib/couchbase/data"
	remoteEndpoint = fmt.Sprintf("http://%s:8091/nodes/self/controller/settings", node.Address)

	respcode, body, err = SendRequest("POST", remoteEndpoint, requestBody, node.Auth)

	if err != nil || respcode != 200 {
		errMsg := fmt.Sprintf("error initializing node : %s", body)
		return fmt.Errorf(errMsg)
	}

	fmt.Println("2: renaming node")
	requestBody = make(map[string]string)
	requestBody["hostname"] = node.Hostname
	remoteEndpoint = fmt.Sprintf("http://%s:8091/node/controller/rename", node.Address)

	respcode, body, err = SendRequest("POST", remoteEndpoint, requestBody, node.Auth)

	if err != nil || respcode != 200 {
		errMsg := fmt.Sprintf("error renaming node : %s", body)
		return fmt.Errorf(errMsg)
	}

	log.Println("4: enabling autofail over")
	requestBody = make(map[string]string)
	requestBody["enabled"] = "true"
	requestBody["timeout"] = "3600"
	remoteEndpoint = fmt.Sprintf("http://%s:8091/settings/autoFailover", node.Address)

	respcode, body, err = SendRequest("POST", remoteEndpoint, requestBody, node.Auth)

	if err != nil || respcode != 200 {
		errMsg := fmt.Sprintf("error initializing node node : %s", body)
		return fmt.Errorf(errMsg)
	}

	log.Println("5: creating default buckets")
	requestBody = make(map[string]string)
	requestBody["flushEnabled"] = "1"
	requestBody["threadsNumber"] = "3"
	requestBody["replicaIndex"] = "0"
	requestBody["replicaNumber"] = "0"
	requestBody["evictionPolicy"] = "valueOnly"
	requestBody["ramQuotaMB"] = "100"
	requestBody["bucketType"] = "membase"
	requestBody["name"] = "default"

	remoteEndpoint = fmt.Sprintf("http://%s:8091/pools/default/buckets", node.Address)

	respcode, body, err = SendRequest("POST", remoteEndpoint, requestBody, node.Auth)
	if err != nil || respcode != 202 {
		errMsg := fmt.Sprintf("error initializing node node : %s", body)
		return fmt.Errorf(errMsg)
	}

	return nil
}

func (node *CouchbaseNode) AddNode(remoteAddress string) error {
	requestBody := make(map[string]string)
	requestBody["hostname"] = node.Hostname
	requestBody["user"] = node.Auth.Username
	requestBody["password"] = node.Auth.Password
	requestBody["services"] = "kv,n1ql,index,fts"
	remoteEndpoint := fmt.Sprintf("http://%s/controller/addNode", remoteAddress)

	respcode, body, err := SendRequest("POST", remoteEndpoint, requestBody, node.Auth)

	if err != nil || respcode != 200 {
		errMsg := fmt.Sprintf("error adding node : %s", body)
		return fmt.Errorf(errMsg)
	}

	return nil
}
