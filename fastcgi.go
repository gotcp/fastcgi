package fastcgi

import (
	"encoding/binary"
	"io"
	"sync"

	"golang.org/x/sys/unix"

	"github.com/gotcp/epollclient"
	"github.com/wuyongjia/bytesbuffer"
)

const (
	FCGI_BEGIN_REQUEST     uint8 = 1
	FCGI_ABORT_REQUEST     uint8 = 2
	FCGI_END_REQUEST       uint8 = 3
	FCGI_PARAMS            uint8 = 4
	FCGI_STDIN             uint8 = 5
	FCGI_STDOUT            uint8 = 6
	FCGI_STDERR            uint8 = 7
	FCGI_DATA              uint8 = 8
	FCGI_GET_VALUES        uint8 = 9
	FCGI_GET_VALUES_RESULT uint8 = 10
	FCGI_UNKNOWN_TYPE      uint8 = 11
)

const (
	FCGI_RESPONDER  uint8 = 1
	FCGI_AUTHORIZER uint8 = 2
	FCGI_FILTER     uint8 = 3
)

const (
	FCGI_REQUEST_COMPLETE uint8 = 0
	FCGI_CANT_MPX_CONN    uint8 = 1
	FCGI_OVERLOADED       uint8 = 2
	FCGI_UNKNOWN_ROLE     uint8 = 3
)

const (
	FCGI_MAX_CONNS  string = "MAX_CONNS"
	FCGI_MAX_REQS   string = "MAX_REQS"
	FCGI_MPXS_CONNS string = "MPXS_CONNS"
)

const (
	FCGI_DEFAULT_VERSION uint8 = 1
	FCGI_KEEP_ALIVE      uint8 = 1
)

const (
	FCGI_HEADER_LENGTH         int    = 8
	FCGI_HEADER_CONTENT_LENGTH int    = FCGI_HEADER_LENGTH * 2
	FCGI_CONTENT_LENGTH        int    = 65520
	FCGI_PAD_LENGTH            int    = 255
	FCGI_BUFFER_LENGTH         int    = FCGI_CONTENT_LENGTH + FCGI_HEADER_LENGTH + FCGI_PAD_LENGTH
	FCGI_MAX_ID                uint16 = 65535
)

var (
	pad [FCGI_PAD_LENGTH]byte
)

const (
	HTTP_CONTENT_TYPE_LENGTH = 256
)

const (
	DEFAULT_KEEP_ALIVE = 1
)

var (
	HTTP_CONTENT_TYPE            = []byte("CONTENT_TYPE")
	HTTP_CONTENT_LENGTH          = []byte("CONTENT_LENGTH")
	HTTP_CONTENT_TYPE_X_WWW_FORM = []byte("application/x-www-form-urlencoded")
)

var (
	HTTP_CONTENT_PREFIX                           = []byte("\r\nContent-Disposition: form-data; name=\"")
	HTTP_MULTIPART_FORM_DATA_PREFIX               = []byte("multipart/form-data; boundary=")
	HTTP_MULTIPART_FORM_FILE_PREFIX               = []byte("; filename=\"")
	HTTP_MULTIPART_FORM_CONTENT_TYPE_PREFIX       = []byte("Content-Type: ")
	HTTP_MULTIPART_FORM_CONTENT_TYPE_OCTET_STREAM = []byte("application/octet-stream")
	DOUBLE_LINES                                  = []byte("--")
	DOUBLE_QUOTES                                 = []byte("\"")
	CRLF                                          = []byte("\r\n")
	CRLF2                                         = []byte("\r\n\r\n")
)

type OpCode uint8

const (
	OPT_NONE     OpCode = 0
	OPT_CONTINUE OpCode = 1
	OPT_BREAK    OpCode = 2
)

type FCGIHeader struct {
	version       uint8
	recordType    uint8
	requestId     uint16
	contentLength uint16
	paddingLength uint8
	reserved      uint8
}

type Fcgi struct {
	id         *ID
	clientPool *epollclient.Connections
	record     Record
	formData   FormData
	OnError    OnErrorEvent
}

type OnReadEvent func(content []byte, n int, err error) OpCode
type OnErrorEvent func(conn *epollclient.Conn, err error)

func New(host string, port int, capacity int) *Fcgi {
	var fcgi = &Fcgi{
		id:         &ID{Id: 1, Lock: &sync.RWMutex{}},
		clientPool: epollclient.New(host, port, capacity),
	}

	fcgi.clientPool.SetBufferLength(FCGI_CONTENT_LENGTH)
	fcgi.clientPool.SetKeepAlive(DEFAULT_KEEP_ALIVE)

	fcgi.record.headerBuffer = bytesbuffer.New(FCGI_BUFFER_LENGTH)

	fcgi.formData.data = bytesbuffer.New(FCGI_BUFFER_LENGTH)
	fcgi.formData.contentType = bytesbuffer.New(HTTP_CONTENT_TYPE_LENGTH)
	fcgi.formData.boundary = nil

	return fcgi
}

func (fcgi *Fcgi) SetKeepAlive(n int) {
	fcgi.clientPool.SetKeepAlive(n)
}

func (fcgi *Fcgi) Close(conn *epollclient.Conn) error {
	return fcgi.clientPool.Close(conn)
}

func (fcgi *Fcgi) Reconnect(conn *epollclient.Conn) error {
	return fcgi.clientPool.Reconnect(conn)
}

func (fcgi *Fcgi) read(conn *epollclient.Conn) (int, error) {
	var ptr = fcgi.record.headerBuffer.GetPtr()

	var _, err = epollclient.Read(conn.Fd, (*ptr)[:FCGI_HEADER_LENGTH])
	if err != nil {
		return -1, err
	}

	fcgi.assignRecordHeader()

	err = fcgi.isRecordOk()
	if err != nil {
		if err == io.EOF {
			epollclient.Read(conn.Fd, fcgi.record.contentBuffer[:])
		}
		return -1, err
	}

	_, err = epollclient.Read(conn.Fd, fcgi.record.contentBuffer[:int(fcgi.record.header.contentLength)+int(fcgi.record.header.paddingLength)])
	if err != nil {
		return -1, err
	}

	return int(fcgi.record.header.contentLength), nil
}

func (fcgi *Fcgi) Get(params [][]byte) (*epollclient.Conn, error) {
	var conn, err = fcgi.request(params, nil, 0, false)
	if err == unix.EPIPE {
		conn, err = fcgi.request(params, nil, 0, true)
	}
	return conn, err
}

func (fcgi *Fcgi) Post(params [][]byte, content []byte) (*epollclient.Conn, error) {
	var contentLength = len(content)
	var conn, err = fcgi.request(params, content, contentLength, false)
	if err == unix.EPIPE {
		conn, err = fcgi.request(params, content, contentLength, true)
	}
	return conn, err
}

func (fcgi *Fcgi) PostFile(params [][]byte, content []byte) (*epollclient.Conn, error) {
	fcgi.writeMultipartFormParams(content)

	Uint32ToBytes(fcgi.formData.contentLength[:], uint32(fcgi.formData.data.GetLength()))

	params = append(params,
		HTTP_CONTENT_TYPE,
		fcgi.formData.contentType.Get(),
		HTTP_CONTENT_LENGTH,
		fcgi.formData.contentLength[:],
	)

	var conn, err = fcgi.request(params, fcgi.formData.data.Get(), fcgi.formData.data.GetLength(), false)

	if err == unix.EPIPE {
		conn, err = fcgi.request(params, fcgi.formData.data.Get(), fcgi.formData.data.GetLength(), true)
	}

	return conn, err
}

func (fcgi *Fcgi) request(params [][]byte, content []byte, contentLength int, reconnect bool) (*epollclient.Conn, error) {
	var conn, err = fcgi.GetConn()
	if err != nil {
		return nil, err
	}

	if reconnect {
		err = fcgi.clientPool.Reconnect(conn)
		if err != nil {
			return nil, err
		}
	}

	fcgi.record.headerBuffer.Reset()

	var requestId = fcgi.id.getId()
	conn.Data = requestId

	err = fcgi.writeBeginRequest(conn, requestId, uint16(FCGI_RESPONDER), FCGI_KEEP_ALIVE)
	if err != nil {
		fcgi.PutConn(conn)
		return nil, err
	}

	err = fcgi.writeParams(conn, requestId, params)
	if err != nil {
		fcgi.PutConn(conn)
		return nil, err
	}

	if contentLength > 0 {
		err = fcgi.writeStdinRequest(conn, requestId, content, contentLength)
		if err != nil {
			fcgi.PutConn(conn)
			return nil, err
		}

		_, err = fcgi.writeRecord(conn, FCGI_STDIN, requestId, nil, 0)
		if err != nil {
			return nil, err
		}
	}

	return conn, nil
}

func (fcgi *Fcgi) Read(conn *epollclient.Conn, onRead OnReadEvent) {
	var err error
	var n int
	var op OpCode
	for {
		n, err = fcgi.read(conn)
		if err == nil {
			op = onRead(fcgi.record.contentBuffer[:], n, nil)
			if op == OPT_BREAK {
				err = fcgi.clientPool.Close(conn)
				if err != nil && fcgi.OnError != nil {
					fcgi.OnError(conn, err)
				}
				return
			} else if op != OPT_CONTINUE {
				return
			}
			if n <= 0 {
				return
			}
		} else {
			op = onRead(nil, -1, err)
			if op == OPT_BREAK {
				err = fcgi.clientPool.Close(conn)
				if err != nil && fcgi.OnError != nil {
					fcgi.OnError(conn, err)
				}
			}
			return
		}
	}
}

func (fcgi *Fcgi) writeParams(conn *epollclient.Conn, requestId uint16, params [][]byte) error {
	var err error
	var k, v []byte
	var l, kl, vl int
	var n int

	var paramsCount = len(params)
	var idx = 0

	for {
		k = params[idx]
		v = params[idx+1]

		kl = len(k)
		vl = len(v)

		l = FCGI_HEADER_LENGTH + kl + vl

		if l > FCGI_CONTENT_LENGTH {
			v = v[:FCGI_CONTENT_LENGTH-FCGI_HEADER_LENGTH-len(k)]

			vl = len(v)
			l = FCGI_HEADER_LENGTH + kl + vl
		}

		n = 0
		n += writeParamSize(fcgi.record.contentBuffer[n:], uint32(len(k)))
		n += writeParamSize(fcgi.record.contentBuffer[n:], uint32(len(v)))

		copy(fcgi.record.contentBuffer[n:], k)
		n += kl

		copy(fcgi.record.contentBuffer[n:], v)
		n += vl

		_, err = fcgi.writeRecord(conn, FCGI_PARAMS, requestId, fcgi.record.contentBuffer[:n], n)
		if err != nil {
			return err
		}

		idx += 2
		if idx >= paramsCount {
			break
		}
	}

	_, err = fcgi.writeRecord(conn, FCGI_PARAMS, requestId, nil, 0)
	if err != nil {
		return err
	}

	return nil
}

func writeParamSize(buffer []byte, size uint32) int {
	if size <= 127 {
		buffer[0] = byte(size)
		return 1
	} else {
		size |= 1 << 31
		binary.BigEndian.PutUint32(buffer, size)
		return 4
	}
}
