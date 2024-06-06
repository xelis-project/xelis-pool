// Copyright (C) 2024 XELIS
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as
// published by the Free Software Foundation, either version 3 of the
// License, or (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <https://www.gnu.org/licenses/>.

package main

import (
	"bufio"
	"time"
	"xelpool/config"
	"xelpool/log"
	"xelpool/util"
	"xelpool/xatum"
	"xelpool/xatum/server"
)

func handleXatumConns(s *server.Server) {
	for {
		conn, ok := <-s.NewConnections

		if !ok {
			log.Err("handleXatumConns not OK")
			continue
		}

		ipAddr := util.RemovePort(conn.Conn.RemoteAddr().String())

		log.Info("Xatum miner with IP", ipAddr, "connectioned")

		go handleConn(s, conn)
	}
}

func handleConn(s *server.Server, conn *server.Connection) {
	rdr := bufio.NewReader(conn.Conn)

	packetsRecv := 0

	go sendPingPackets(s, conn)

	for {

		conn.CData.Lock()
		var err error
		if packetsRecv == 0 {
			err = conn.Conn.SetReadDeadline(time.Now().Add(config.TIMEOUT * time.Second))
		} else {
			err = conn.Conn.SetReadDeadline(time.Now().Add(config.SLAVE_MINER_TIMEOUT * time.Second))
		}
		conn.CData.Unlock()

		if err != nil {
			log.Dev(conn.Conn.RemoteAddr().String(), "deadline err", err)
			conn.Conn.Close()
			return
		}

		str, err := rdr.ReadString('\n')

		packetsRecv++

		if err != nil {
			s.Lock()
			s.Kick(conn.Id)
			s.Unlock()
			return
		}

		log.Net("received <<<", str)

		var jobToSend JobToSend

		print, shouldKick, err := handleConnPacket(&conn.CData, str, packetsRecv, conn.Conn.RemoteAddr().String(), &jobToSend, [16]byte{})
		if err != nil {
			log.Warn("Xatum:", err)
		}
		if print != nil {
			conn.CData.Lock()
			conn.Send(xatum.PacketS2C_Print, print)
			conn.CData.Unlock()
		}
		if shouldKick {
			s.Lock()
			s.Kick(conn.Id)
			s.Unlock()
		}

		if jobToSend.Diff != 0 {
			log.Debug("jobToSend has diff", jobToSend.Diff)
			SendJob(conn, jobToSend.Diff, jobToSend.BM)
		}

	}

}
