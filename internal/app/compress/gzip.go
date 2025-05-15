package compress

import (
	"compress/gzip"
	"io"
	"net/http"
	"strings"
)

type CompressWriter struct {
	writer     http.ResponseWriter
	gzipWriter *gzip.Writer
}

func NewCompressWriter(writer http.ResponseWriter) *CompressWriter {
	return &CompressWriter{
		writer:     writer,
		gzipWriter: gzip.NewWriter(writer),
	}
}

func (c *CompressWriter) Header() http.Header {
	return c.writer.Header()
}

func (c *CompressWriter) Write(p []byte) (int, error) {
	if c.ShouldCompress() {
		return c.gzipWriter.Write(p)
	}
	return c.writer.Write(p)
}

func (c *CompressWriter) WriteHeader(statusCode int) {
	if c.ShouldCompress() {
		c.writer.Header().Set("Content-Encoding", "gzip")
	}
	c.writer.WriteHeader(statusCode)
}

func (c *CompressWriter) ShouldCompress() bool {
	contentType := c.Header().Get("Content-Type")
	if strings.Contains(contentType, "application/json") || strings.Contains(contentType, "text/html") {
		return true
	}
	return false
}

func (c *CompressWriter) Close() error {
	if c.ShouldCompress() {
		return c.gzipWriter.Close()
	}
	return nil
}

type CompressReader struct {
	reader     io.ReadCloser
	gzipReader *gzip.Reader
}

func NewCompressReader(reader io.ReadCloser) (*CompressReader, error) {
	gzipReader, err := gzip.NewReader(reader)
	if err != nil {
		return nil, err
	}

	return &CompressReader{
		reader:     reader,
		gzipReader: gzipReader,
	}, nil
}

func (c *CompressReader) Read(p []byte) (n int, err error) {
	return c.gzipReader.Read(p)
}

func (c *CompressReader) Close() error {
	if err := c.reader.Close(); err != nil {
		return err
	}
	return c.gzipReader.Close()
}
