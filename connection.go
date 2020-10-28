package fastcgi

import (
	"github.com/gotcp/epollclient"
)

func (fcgi *Fcgi) GetConn() (*epollclient.Conn, error) {
	return fcgi.clientPool.Get()
}

func (fcgi *Fcgi) PutConn(conn *epollclient.Conn) {
	fcgi.clientPool.Put(conn)
}
