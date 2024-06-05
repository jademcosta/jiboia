package compressor

import "io"

type NoopCompressorWriter struct {
	buffer io.Writer
}

func NewNoopCompressorWriter(buf io.Writer) CompressorWriter {
	return &NoopCompressorWriter{buffer: buf}
}

func (c *NoopCompressorWriter) Write(p []byte) (int, error) {
	return c.buffer.Write(p)
}

func (c *NoopCompressorWriter) Close() error {
	return nil
}

func (c *NoopCompressorWriter) Flush() error {
	return nil
}

type NoopCompressorReader struct {
	reader io.Reader
}

func NewNoopCompressorReader(r io.Reader) CompressorReader {
	return &NoopCompressorReader{reader: r}
}

func (c *NoopCompressorReader) Close() error {
	return nil
}

func (c *NoopCompressorReader) Read(p []byte) (int, error) {
	return c.reader.Read(p)
}
