package main

import (
	"net/http"
)

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
			if resp.Code == 200 {
				c[r.URL.String()] = cacheEntry{resp}
			}
			resp.WriteResponse(rw)
		}
		// TODO bookkeeping for the cache here
	})
}
