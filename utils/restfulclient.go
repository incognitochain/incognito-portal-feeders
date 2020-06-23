package utils

import (
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"time"
)

type RestfulClient struct {
	client  *http.Client
	apiKey string
}

func NewRestfulClient() *RestfulClient {
	httpClient := &http.Client{
		Timeout: time.Second * 60,
	}

	return &RestfulClient{
		client:  httpClient,
	}
}

func (r *RestfulClient) Get(
	link string,
	header map[string]string,
	queryString map[string]string,
) ([]byte, error) {
	req, err := http.NewRequest("GET", link, nil)
	if err != nil {
		log.Print(err)
		return nil, err
	}

	req.Header.Set("Accepts", "application/json")

	if len(header) > 0 {
		for key, value := range header {
			req.Header.Add(key, value)
		}
	}

	if len(queryString) > 0 {
		q := url.Values{}
		for key, value := range queryString {
			q.Add(key, value)
		}
		req.URL.RawQuery = q.Encode()
	}

	resp, err := r.client.Do(req)
	if err != nil {
		fmt.Println("RestfulClient: Error sending request to server")
		return nil, err
	}
	defer resp.Body.Close()

	respBody, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Println("RestfulClient: Error reading body. ", err)
		return nil, err
	}

	if resp.StatusCode != 200 {
		return nil, errors.New("RestfulClient: Get api " + link + " error, status: " + resp.Status + ", body: " + string(respBody))
	}

	return respBody, nil
}
