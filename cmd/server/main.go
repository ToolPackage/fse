package main

import (
	log "github.com/Luncert/slog"
	"github.com/ToolPackage/fse/tx"
	"net"
)

func main() {
	log.InitLogger("cmd/conf/log.yml")

	//s := server.New()
	//s.Start()

	addr, _ := net.ResolveTCPAddr("tcp", "127.0.0.1:9330")
	listener, _ := net.ListenTCP("tcp", addr)
	defer listener.Close()
	for {
		conn, err := listener.AcceptTCP()
		if err != nil {
			log.Error(err)
			continue
		}
		c := tx.NewChannel(conn, conn)
		go c.Process()
	}
}
