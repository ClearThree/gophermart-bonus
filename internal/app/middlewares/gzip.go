package middlewares

import (
	"github.com/ClearThree/gophermart-bonus/internal/app/compress"
	"github.com/ClearThree/gophermart-bonus/internal/app/logger"
	"net/http"
	"strings"
)

func GzipMiddleware(next http.Handler) http.Handler {
	fn := func(writer http.ResponseWriter, request *http.Request) {
		usedWriter := writer

		acceptEncoding := request.Header.Get("Accept-Encoding")
		supportsGzip := strings.Contains(acceptEncoding, "gzip")
		if supportsGzip {
			compressWriter := compress.NewCompressWriter(writer)
			usedWriter = compressWriter

			defer func(compressWriter *compress.CompressWriter) {
				err := compressWriter.Close()
				if err != nil {
					logger.Log.Errorf("error closing compressWriter: %v", err)
				}
			}(compressWriter)
		}

		contentEncoding := request.Header.Get("Content-Encoding")
		sendsGzip := strings.Contains(contentEncoding, "gzip")
		if sendsGzip {
			compressReader, err := compress.NewCompressReader(request.Body)
			if err != nil {
				writer.WriteHeader(http.StatusInternalServerError)
				return
			}
			request.Body = compressReader
			defer func(compressReader *compress.CompressReader) {
				innerErr := compressReader.Close()
				if innerErr != nil {
					logger.Log.Errorf("error closing compressReader: %v", innerErr)
				}
			}(compressReader)
		}

		next.ServeHTTP(usedWriter, request)
	}
	return http.HandlerFunc(fn)
}
