package fastcgi

import (
	"bytes"
	"fmt"
	"io"

	"crypto/rand"

	"github.com/gotcp/epollclient"
	"github.com/wuyongjia/bytesbuffer"
)

type FormData struct {
	data          *bytesbuffer.Buffer
	contentType   *bytesbuffer.Buffer
	boundary      []byte
	contentLength [4]byte // uint32
}

var (
	AND_MARK   = []byte("&")
	EQUAL_MARK = []byte("=")
)

func (fcgi *Fcgi) WriteFormData(conn *epollclient.Conn, content []byte, contentLength int) error {
	var requestId, ok = conn.Data.(uint16)
	if ok == false {
		return ErrorGetRequestId
	}
	return fcgi.writeStdinRequest(conn, requestId, content, contentLength)
}

// content: k1=v1&k2=v2&k3=v3 ...
func (fcgi *Fcgi) WriteMultipartFormData(conn *epollclient.Conn, content []byte, contentLength int) error {
	var err error

	var requestId, ok = conn.Data.(uint16)
	if ok == false {
		return ErrorGetRequestId
	}

	if fcgi.formData.boundary == nil {
		err = fcgi.initMultipartFormBoundary()
		if err != nil {
			return err
		}
	}

	fcgi.writeMultipartFormParams(content)

	return fcgi.writeStdinRequest(conn, requestId, fcgi.formData.data.Get(), fcgi.formData.data.GetLength())
}

func (fcgi *Fcgi) WriteMultipartFormFileContent(conn *epollclient.Conn, content []byte, name []byte, filename []byte) error {
	var requestId, ok = conn.Data.(uint16)
	if ok == false {
		return ErrorGetRequestId
	}

	fcgi.writeMultipartFormFileContent(content, name, filename)
	fcgi.writeMultipartFormFileEnd()

	return fcgi.writeStdinRequest(conn, requestId, fcgi.formData.data.Get(), fcgi.formData.data.GetLength())
}

func (fcgi *Fcgi) WriteMultipartFormFileEnd(conn *epollclient.Conn) error {
	var requestId, ok = conn.Data.(uint16)
	if ok == false {
		return ErrorGetRequestId
	}

	fcgi.formData.data.Reset()
	fcgi.writeMultipartFormFileEnd()

	return fcgi.writeStdinRequest(conn, requestId, fcgi.formData.data.Get(), fcgi.formData.data.GetLength())
}

func (fcgi *Fcgi) GetMultipartFormBoundary() []byte {
	return fcgi.formData.boundary
}

func (fcgi *Fcgi) initMultipartFormBoundary() error {
	var buf [30]byte
	var _, err = io.ReadFull(rand.Reader, buf[:])
	if err != nil {
		return err
	}

	fcgi.formData.boundary = []byte(fmt.Sprintf("%x", buf[:]))

	fcgi.formData.contentType.Reset()
	fcgi.formData.contentType.Write(HTTP_MULTIPART_FORM_DATA_PREFIX)
	fcgi.formData.contentType.Write(fcgi.formData.boundary)

	return nil
}

func (fcgi *Fcgi) writeMultipartFormParams(content []byte) error {
	fcgi.formData.data.Reset()

	if len(content) > 0 {
		var err = fcgi.initMultipartFormBoundary()
		if err != nil {
			return err
		}

		var formParams = bytes.Split(content, AND_MARK)
		var paramsCount = len(formParams)
		var i int
		var kv [][]byte

		for i = 0; i < paramsCount; i++ {
			kv = bytes.Split(formParams[i], EQUAL_MARK)
			if len(kv) == 2 {
				fcgi.formData.data.Write(DOUBLE_LINES)
				fcgi.formData.data.Write(fcgi.formData.boundary)

				fcgi.formData.data.Write(HTTP_CONTENT_PREFIX)
				fcgi.formData.data.Write(kv[0])
				fcgi.formData.data.Write(DOUBLE_QUOTES)

				fcgi.formData.data.Write(CRLF2)

				fcgi.formData.data.Write(kv[1])

				fcgi.formData.data.Write(CRLF)
			}
		}
	}

	return nil
}

func (fcgi *Fcgi) writeMultipartFormFileContent(content []byte, name []byte, filename []byte) {
	fcgi.formData.data.Reset()

	fcgi.formData.data.Write(DOUBLE_LINES)
	fcgi.formData.data.Write(fcgi.formData.boundary)

	fcgi.formData.data.Write(HTTP_CONTENT_PREFIX)
	fcgi.formData.data.Write(name)
	fcgi.formData.data.Write(DOUBLE_QUOTES)

	fcgi.formData.data.Write(HTTP_MULTIPART_FORM_FILE_PREFIX)
	fcgi.formData.data.Write(filename)
	fcgi.formData.data.Write(DOUBLE_QUOTES)

	fcgi.formData.data.Write(CRLF)

	fcgi.formData.data.Write(HTTP_MULTIPART_FORM_CONTENT_TYPE_PREFIX)
	fcgi.formData.data.Write(HTTP_MULTIPART_FORM_CONTENT_TYPE_OCTET_STREAM)

	fcgi.formData.data.Write(CRLF2)

	fcgi.formData.data.Write(content)

	fcgi.formData.data.Write(CRLF)
}

func (fcgi *Fcgi) writeMultipartFormFileEnd() {
	fcgi.formData.data.Write(DOUBLE_LINES)
	fcgi.formData.data.Write(fcgi.formData.boundary)
	fcgi.formData.data.Write(DOUBLE_LINES)
	fcgi.formData.data.Write(CRLF)
}
