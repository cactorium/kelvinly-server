package main

import (
	"net/http"
)

type Response struct {
	Code    int
	Headers map[string][]string
	Body    []byte
}

func (r Response) WriteResponse(rw http.ResponseWriter) {
	for k, vs := range r.Headers {
		for _, v := range vs {
			rw.Header().Add(k, v)
		}
	}
	rw.WriteHeader(r.Code)
	rw.Write(r.Body)
}

// implements ResponseWriter to collect HTTP responses
type ResponseCollector struct {
	Response
}

func (rc *ResponseCollector) Header() http.Header {
	if rc.Headers == nil {
		rc.Headers = make(map[string][]string)
	}
	return rc.Headers
}

func (rc *ResponseCollector) Write(bs []byte) (int, error) {
	if rc.Code == 0 {
		rc.Code = 200
	}
	rc.Body = append(rc.Body, bs...)
	return len(bs), nil
}

func (rc *ResponseCollector) WriteHeader(code int) {
	rc.Code = code
}

func (rc *ResponseCollector) CollectResponse() Response {
	return rc.Response
}
