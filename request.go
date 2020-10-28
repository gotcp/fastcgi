package fastcgi

import (
	"github.com/gotcp/epollclient"
)

func (fcgi *Fcgi) writeStdinRequest(conn *epollclient.Conn, requestId uint16, content []byte, contentLength int) error {
	var err error

	if contentLength <= 0 {
		_, err = fcgi.writeRecord(conn, FCGI_STDIN, requestId, nil, 0)
		if err != nil {
			return err
		}
		return nil
	}

	var n int
	var startIndex, endIndex, length = GetNextIndex(-1, FCGI_CONTENT_LENGTH, contentLength)

	for {
		n, err = fcgi.writeRecord(conn, FCGI_STDIN, requestId, content[startIndex:endIndex], length)
		if err != nil {
			return err
		}

		startIndex += (n - FCGI_HEADER_LENGTH)
		startIndex, endIndex, length = GetNextIndex(startIndex, FCGI_CONTENT_LENGTH, contentLength)

		if startIndex < 0 {
			break
		}
	}

	return nil
}

func (fcgi *Fcgi) writeBeginRequest(conn *epollclient.Conn, requestId uint16, role uint16, flags uint8) error {
	fcgi.initRecordHeader(FCGI_BEGIN_REQUEST, requestId, FCGI_HEADER_LENGTH)
	fcgi.writeRecordHeader()
	fcgi.writeBeginRequestRecordContent(role, flags)
	var _, err = fcgi.Write(conn, fcgi.record.headerBuffer.Get())
	return err
}

func (fcgi *Fcgi) WriteEndRequest(conn *epollclient.Conn, appStatus uint32, protocolStatus uint8) error {
	var requestId, ok = conn.Data.(uint16)
	if ok == false {
		return ErrorGetRequestId
	}
	fcgi.initRecordHeader(FCGI_END_REQUEST, requestId, FCGI_HEADER_LENGTH)
	fcgi.writeRecordHeader()
	fcgi.writeEndRequestRecordContent(appStatus, protocolStatus)
	var _, err = fcgi.Write(conn, fcgi.record.headerBuffer.Get())
	return err
}
