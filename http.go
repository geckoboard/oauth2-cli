package main

import (
	"bytes"
	"io/ioutil"
	"log"
	"net/http"
	"time"
)

type loggingTransport struct {
	Transport http.RoundTripper
}

func (l loggingTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	reqBody, err := ioutil.ReadAll(r.Body)
	if err != nil {
		return nil, err
	}
	r.Body = ioutil.NopCloser(bytes.NewReader(reqBody))

	start := time.Now()
	log.Printf("request: %s %s\nbody:\n%s\n", r.Method, r.URL, string(reqBody))

	res, err := l.Transport.RoundTrip(r)
	duration := time.Since(start)
	if err != nil {
		log.Printf("error: %s in %s\n", err, duration)
	} else {
		resBody, err := ioutil.ReadAll(res.Body)
		if err != nil {
			return nil, err
		}
		res.Body = ioutil.NopCloser(bytes.NewReader(resBody))
		log.Printf("response: %d in %s\nbody:\n%s\n", res.StatusCode, duration, resBody)
	}
	return res, err
}
