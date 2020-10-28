package fastcgi

import (
	"encoding/binary"
	"io"

	"github.com/gotcp/epollclient"
	"github.com/wuyongjia/bytesbuffer"
)

type Record struct {
	header        FCGIHeader
	headerBuffer  *bytesbuffer.Buffer
	contentBuffer [FCGI_BUFFER_LENGTH]byte
}

func (fcgi *Fcgi) writeRecord(conn *epollclient.Conn, recordType uint8, requestId uint16, content []byte, contentLength int) (int, error) {
	fcgi.initRecordHeader(recordType, requestId, contentLength)
	fcgi.writeRecordHeader()
	if contentLength > 0 {
		fcgi.writeRecordContent(content)
	}
	fcgi.writeRecordPad()
	return fcgi.Write(conn, fcgi.record.headerBuffer.Get())
}

func (fcgi *Fcgi) isRecordOk() error {
	if fcgi.record.header.version != FCGI_DEFAULT_VERSION {
		return ErrorInvalidFcgiVertion
	}
	if fcgi.record.header.recordType == FCGI_END_REQUEST {
		return io.EOF
	}
	return nil
}

func (fcgi *Fcgi) initRecordHeader(recordType uint8, requestId uint16, contentLength int) {
	fcgi.record.header.version = FCGI_DEFAULT_VERSION
	fcgi.record.header.recordType = recordType
	fcgi.record.header.requestId = requestId
	fcgi.record.header.contentLength = uint16(contentLength)
	fcgi.record.header.paddingLength = uint8(-contentLength & 7)
	fcgi.record.header.reserved = 0
}

func (fcgi *Fcgi) writeRecordContent(content []byte) {
	fcgi.record.headerBuffer.WriteWithIndex(FCGI_HEADER_LENGTH, content)
}

func (fcgi *Fcgi) writeBeginRequestRecordContent(role uint16, flags uint8) {
	var ptr = fcgi.record.headerBuffer.GetPtr()

	(*ptr)[FCGI_HEADER_LENGTH] = byte(role >> 8)
	(*ptr)[FCGI_HEADER_LENGTH+1] = byte(role)
	(*ptr)[FCGI_HEADER_LENGTH+2] = flags
	(*ptr)[FCGI_HEADER_LENGTH+3] = byte(0)
	(*ptr)[FCGI_HEADER_LENGTH+4] = byte(0)
	(*ptr)[FCGI_HEADER_LENGTH+5] = byte(0)
	(*ptr)[FCGI_HEADER_LENGTH+6] = byte(0)
	(*ptr)[FCGI_HEADER_LENGTH+7] = byte(0)

	fcgi.record.headerBuffer.SetIndex(FCGI_HEADER_CONTENT_LENGTH)
}

func (fcgi *Fcgi) writeEndRequestRecordContent(appStatus uint32, protocolStatus uint8) {
	var ptr = fcgi.record.headerBuffer.GetPtr()

	(*ptr)[FCGI_HEADER_LENGTH] = byte(appStatus >> 8)
	(*ptr)[FCGI_HEADER_LENGTH+1] = byte(appStatus >> 16)
	(*ptr)[FCGI_HEADER_LENGTH+2] = byte(appStatus >> 24)
	(*ptr)[FCGI_HEADER_LENGTH+3] = byte(appStatus)
	(*ptr)[FCGI_HEADER_LENGTH+4] = protocolStatus
	(*ptr)[FCGI_HEADER_LENGTH+5] = byte(0)
	(*ptr)[FCGI_HEADER_LENGTH+6] = byte(0)
	(*ptr)[FCGI_HEADER_LENGTH+7] = byte(0)

	fcgi.record.headerBuffer.SetIndex(FCGI_HEADER_CONTENT_LENGTH)
}

func (fcgi *Fcgi) writeRecordHeader() {
	fcgi.record.headerBuffer.Reset()

	var ptr = fcgi.record.headerBuffer.GetPtr()

	(*ptr)[0] = fcgi.record.header.version
	(*ptr)[1] = fcgi.record.header.recordType

	(*ptr)[2] = byte(fcgi.record.header.requestId >> 8)
	(*ptr)[3] = byte(fcgi.record.header.requestId)

	(*ptr)[4] = byte(fcgi.record.header.contentLength >> 8)
	(*ptr)[5] = byte(fcgi.record.header.contentLength)

	(*ptr)[6] = fcgi.record.header.paddingLength
	(*ptr)[7] = byte(0)

	fcgi.record.headerBuffer.SetIndex(FCGI_HEADER_LENGTH)
}

func (fcgi *Fcgi) writeRecordPad() {
	fcgi.record.headerBuffer.Write(pad[:fcgi.record.header.paddingLength])
}

func (fcgi *Fcgi) assignRecordHeader() {
	var ptr = fcgi.record.headerBuffer.GetPtr()

	fcgi.record.header.version = (*ptr)[0]
	fcgi.record.header.recordType = (*ptr)[1]
	fcgi.record.header.requestId = binary.BigEndian.Uint16((*ptr)[2:4])
	fcgi.record.header.contentLength = binary.BigEndian.Uint16((*ptr)[4:6])
	fcgi.record.header.paddingLength = (*ptr)[6]
	fcgi.record.header.reserved = 0
}
