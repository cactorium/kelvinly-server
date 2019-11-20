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
	return rc.Headers
}

func (rc *ResponseCollector) Write(bs []byte) (int, error) {
	rc.Body = append(rc.Body, bs...)
	return len(bs), nil
}

func (rc *ResponseCollector) WriteHeader(code int) {
	rc.Code = code
}

func (rc *ResponseCollector) CollectResponse() Response {
	return rc.Response
}

type cacheEntry struct {
	r Response
}

func Cache(h http.Handler) http.Handler {
	c := make(map[string]cacheEntry)

	return http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			rw.WriteHeader(403)
			rw.Write([]byte("invalid request type"))
			return
		}

		entry, exists := c[r.URL.String()]
		if exists {
			entry.r.WriteResponse(rw)
		} else {
			rc := ResponseCollector{}
			// copy request in case they modify it
			req := *r
			h.ServeHTTP(&rc, &req)
			resp := rc.CollectResponse()
			c[r.URL.String()] = cacheEntry{resp}
			resp.WriteResponse(rw)
		}
		// TODO bookkeeping for the cache here
	})
}
