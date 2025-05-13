package middlewares

import (
	"github.com/ClearThree/gophermart-bonus/internal/app/logger"
	"net/http"
	"strconv"
)

const maxPayloadSize = 1024 * 1024

func ValidationMiddleware(next http.Handler) http.Handler {
	fn := func(writer http.ResponseWriter, request *http.Request) {
		contentLength := request.Header.Get("Content-Length")
		if contentLength != "" {
			contentLengthValue, err := strconv.Atoi(request.Header.Get("Content-Length"))
			if err != nil {
				logger.Log.Warnf("Invalid content length: %s", request.Header.Get("Content-Length"))
				http.Error(writer, "Content-Length header is invalid, should be integer", http.StatusBadRequest)
				return
			}
			if contentLengthValue > maxPayloadSize {
				logger.Log.Warnf("Content is too large: %d", contentLength)
				http.Error(writer, "Content is too large", http.StatusBadRequest)
				return
			}
		}

		next.ServeHTTP(writer, request)
	}
	return http.HandlerFunc(fn)
}
