package couchbase

import (
	"bytes"
	"encoding/gob"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"net/url"
	"os/exec"
	"strings"
	"time"
)

const (
	HELLO      byte = 0x01 // hello byte
	HELLOREPLY byte = 0x02 // reply byte
)

type Auth struct {
	Username string
	Password string
}

type Packet struct {
	Header  byte
	Payload []byte
}

func IPAddr() string {
	hostname := HostName()
	addrs, err := net.LookupIP(hostname)
	if err != nil {
		panic(err)
	}

	for _, addr := range addrs {
		if ipv4 := addr.To4(); ipv4 != nil {
			if !ipv4.IsLoopback() {
				return ipv4.String()
			}
		}
	}
	return ""
}

func HostName() string {
	cmd := exec.Command("/bin/hostname", "-f")
	var out bytes.Buffer
	cmd.Stdout = &out
	err := cmd.Run()
	if err != nil {
		panic(err)
	}
	fqdn := out.String()
	fqdn = fqdn[:len(fqdn)-1]
	return fqdn
}

func Encode(dataStructure interface{}) (encoded []byte, err error) {
	var buffer bytes.Buffer
	encoder := gob.NewEncoder(&buffer)
	err = encoder.Encode(dataStructure)

	if err != nil {
		log.Printf("Error : %s \n", err)
		return nil, err
	}

	return buffer.Bytes(), nil
}

func Decode(dataStructure interface{}, data []byte) error {
	buffer := bytes.NewBuffer(data)
	decoder := gob.NewDecoder(buffer)
	err := decoder.Decode(dataStructure)

	if err != nil {
		log.Println(err)
		return err
	}

	return nil
}

func SendRequest(verb string, remoteEndpoint string, requestBody map[string]string, auth Auth) (respCode int, response string, err error) {
	rawBody := make([]string, 0)

	for key, value := range requestBody {
		data := fmt.Sprintf("%s=%s", key, url.QueryEscape(value))
		rawBody = append(rawBody, data)
	}
	body := strings.Join(rawBody, "&")
	reader := strings.NewReader(body)
	req, err := http.NewRequest(verb, remoteEndpoint, reader)

	if err != nil {
		log.Fatalln("An error occured while performing this request")
		log.Fatal(err)
		return 0, "", err
	}

	req.SetBasicAuth(auth.Username, auth.Password)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	httpClient := http.Client{
		Timeout: time.Second * 30,
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		log.Println("An error occured while perfoming this request")
		log.Fatal(err)
		return 0, "", err
	}

	defer resp.Body.Close()

	respBody, err := ioutil.ReadAll(resp.Body)
	return resp.StatusCode, string(respBody), nil
}
