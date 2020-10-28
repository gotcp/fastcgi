package fastcgi

import (
	"github.com/gotcp/epollclient"
)

func (fcgi *Fcgi) Write(conn *epollclient.Conn, buffer []byte) (int, error) {
	return epollclient.Write(conn.Fd, buffer)
}

func CreateParams(params ...[]byte) [][]byte {
	return params
}

func GetNextIndex(startIndex int, segmentLength int, length int) (int, int, int) {
	var s, e, l int
	if startIndex >= 0 {
		if length > startIndex {
			s = startIndex
			e = startIndex + segmentLength
			if e > length {
				e = length
				l = e - s
			} else {
				l = segmentLength
			}
		} else {
			return -1, -1, -1
		}
	} else {
		s = 0
		if length > segmentLength {
			e = segmentLength
		} else {
			e = length
		}
		l = e
	}
	return s, e, l
}

func Uint32ToBytes(buffer []byte, value uint32) {
	buffer[0] = byte(value >> 8)
	buffer[1] = byte(value >> 16)
	buffer[2] = byte(value >> 24)
	buffer[3] = byte(value)
}
