package mgmtconn

type baseconn interface {
	MakeMgmtConn(socket string)
	Conn()
}
