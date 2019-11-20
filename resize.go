package main

import (
	"bytes"
	"image/jpeg"
	"image/png"
	"net/http"

	"github.com/nfnt/resize"
)

func Resize(maxWidth uint, h http.Handler) http.Handler {
	return http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
		rc := ResponseCollector{}
		req := *r
		h.ServeHTTP(&rc, &req)
		imageResp := rc.CollectResponse()

		if imageResp.Code != 200 {
			imageResp.WriteResponse(rw)
			return
		}

		typ, hasType := imageResp.Headers["content-type"]
		if !hasType || len(typ) == 0 {
			rw.WriteHeader(501)
			rw.Write([]byte("could not determine content type of image"))
			return
		}

		buf := bytes.NewBuffer(imageResp.Body)
		switch typ[0] {
		case "image/png":
			image, err := png.Decode(buf)
			if err != nil {
				rw.WriteHeader(501)
				rw.Write([]byte("error while decoding png: " + err.Error()))
				return
			}
			resizedImage := resize.Thumbnail(maxWidth, 0, image, resize.Lanczos3)
			resizedBuf := new(bytes.Buffer)
			if encodeErr := png.Encode(resizedBuf, resizedImage); encodeErr != nil {
				rw.WriteHeader(501)
				rw.Write([]byte("error while encoding png: " + err.Error()))
				return
			}
			rw.Header().Add("Content-Type", "image/png")
			rw.Write(resizedBuf.Bytes())
		case "image/jpeg":
			image, err := jpeg.Decode(buf)
			if err != nil {
				rw.WriteHeader(501)
				rw.Write([]byte("error while decoding png: " + err.Error()))
				return
			}
			resizedImage := resize.Thumbnail(maxWidth, 0, image, resize.Lanczos3)
			resizedBuf := new(bytes.Buffer)
			jpegOptions := jpeg.Options{Quality: 75}
			if encodeErr := jpeg.Encode(resizedBuf, resizedImage, &jpegOptions); encodeErr != nil {
				rw.WriteHeader(501)
				rw.Write([]byte("error while encoding png: " + err.Error()))
				return
			}
			rw.Header().Add("Content-Type", "image/png")
			rw.Write(resizedBuf.Bytes())
		case "text/html":
			rw.WriteHeader(415)
			rw.Write([]byte("can't resize html files"))
			return
		default:
			rw.WriteHeader(501)
			rw.Write([]byte("unimplemented"))
			return
		}
	})
}
