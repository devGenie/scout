package scoutcore

import (
	"bytes"
	"encoding/gob"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
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
	hostname, err := os.Hostname()
	if err != nil {
		panic(err)
	}
	return hostname
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

func SendRequest(verb string, remoteEndpoint string, requestBody map[string]string, auth Auth) error {
	rawBody := make([]string, 0)

	for key, value := range requestBody {
		data := fmt.Sprintf("%s=%s", key, value)
		data = url.QueryEscape(data)
		rawBody = append(rawBody, data)
	}
	body := strings.Join(rawBody, "&")
	fmt.Println(body)
	reader := strings.NewReader(body)
	req, err := http.NewRequest(verb, remoteEndpoint, reader)

	if err != nil {
		log.Fatalln("An error occured while performing this request")
		log.Fatal(err)
		return err
	}

	req.SetBasicAuth(auth.Username, auth.Password)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := http.DefaultClient.Do(req)

	if err != nil {
		log.Println("An error occured while perfoming this request")
		log.Fatal(err)
		return err
	}

	log.Println(resp)
	defer resp.Body.Close()
	return nil
}
