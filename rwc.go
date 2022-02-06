package kish

import (
	"io"

	"github.com/gorilla/websocket"
)

// Make gorrilla.websocket ReadWriteCloser
// from https://github.com/gorilla/websocket/issues/282
type rwc struct {
	reader io.Reader
	conn   *websocket.Conn
}

func (c *rwc) Write(p []byte) (int, error) {
	err := c.conn.WriteMessage(websocket.BinaryMessage, p)
	if err != nil {
		return 0, err
	}
	return len(p), nil
}

func (c *rwc) Read(p []byte) (int, error) {
	for {
		if c.reader == nil {
			// Advance to next message.
			var err error
			_, c.reader, err = c.conn.NextReader()
			if err != nil {
				return 0, err
			}
		}
		n, err := c.reader.Read(p)
		if err == io.EOF {
			// At end of message.
			c.reader = nil
			if n > 0 {
				return n, nil
			} else {
				// No data read, continue to next message.
				continue
			}
		}
		return n, err
	}
}

func (c *rwc) Close() error {
	return c.conn.Close()
}

func MakeRWC(conn *websocket.Conn) io.ReadWriteCloser {
	return &rwc{conn: conn}
}
