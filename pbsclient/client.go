package pbsclient

import (
	"fmt"
	"io/ioutil"
	"net/http"
)

type Client struct {
	Host string
}

func New(host string) *Client {
	return &Client{Host: host}
}

func (c *Client) Ping() (string, error) {
	url := fmt.Sprintf("%s/api2/json/ping", c.Host)
	resp, err := http.Get(url)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	return string(body), nil
}
