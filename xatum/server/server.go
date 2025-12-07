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

package server

import (
	"crypto/tls"
	"encoding/json"
	"net"
	"strconv"
	"sync"
	"time"
	"xelis-pool/cfg"
	"xelis-pool/log"
	"xelis-pool/pow"
	rate_limit "xelis-pool/rate_limit"
	"xelis-pool/util"
	"xelis-pool/xatum"
)

type Server struct {
	Connections []*Connection

	NewConnections chan *Connection

	sync.RWMutex
}

type Connection struct {
	Conn net.Conn
	Id   uint64

	CData CData
}

type CData struct {
	Jobs []ConnJob

	NextDiff  float64
	LastShare time.Time // in unix milliseconds
	Score     int32
	Wallet    string

	sync.RWMutex
}

func NewCData() CData {
	return CData{
		LastShare: time.Now(),
		NextDiff:  float64(cfg.Cfg.Slave.InitialDifficulty),
		Jobs:      make([]ConnJob, 0, 5),
	}
}

func (c *CData) LastJob() ConnJob {
	if len(c.Jobs) == 0 {
		return ConnJob{}
	}

	return c.Jobs[len(c.Jobs)-1]
}

func (c *Connection) LastJob() ConnJob {
	return c.CData.LastJob()
}

func (c *CData) GetNextDiff() float64 {
	d := c.NextDiff

	seconds := time.Since(c.LastShare).Seconds()

	if seconds > 1 {
		// difficulty halves every 40 seconds
		d /= 1 + (seconds / 40)
	}

	if d < float64(cfg.Cfg.Slave.MinDifficulty) {
		return float64(cfg.Cfg.Slave.MinDifficulty)
	}

	return d
}

type ConnJob struct {
	Diff      uint64
	ChainDiff uint64

	BlockMiner pow.BlockMiner

	SubmittedNonces []uint64
}

func (c *Connection) Send(name string, a any) error {
	data, err := json.Marshal(a)
	if err != nil {
		panic(err)
	}
	return c.SendBytes(append([]byte(name+"~"), data...))
}
func (c *Connection) SendBytes(data []byte) error {
	log.Net(">>>", string(data))
	c.Conn.SetWriteDeadline(time.Now().Add(20 * time.Second))
	_, err := c.Conn.Write(append(data, '\n'))
	if err != nil {
		return err
	}
	return nil
}
func (c *Connection) SendJob(job xatum.S2C_Job) {
	c.Send(xatum.PacketS2C_Job, job)
}

var Cert tls.Certificate

func (s *Server) Start(port uint16) {
	s.NewConnections = make(chan *Connection, 1)

	var err error
	Cert, err = tls.LoadX509KeyPair("cert.pem", "key.pem")
	if err != nil {
		log.Err("Invalid TLS certificate:", err, "generating a new one")

		certPem, keyPem, err := GenCertificate()
		if err != nil {
			log.Fatal(err)
		}

		Cert, err = tls.X509KeyPair(certPem, keyPem)
		if err != nil {
			log.Fatal(err)
		}
	}

	listener, err := tls.Listen("tcp", "0.0.0.0:"+strconv.FormatUint(uint64(port), 10), &tls.Config{
		Certificates: []tls.Certificate{
			Cert,
		},
	})
	if err != nil {
		log.Fatal(err)
	}

	log.Info("Xatum server listening on port", port)

	for {
		c, err := listener.Accept()
		if err != nil {
			log.Err(err)
			continue
		}
		minerIp := util.RemovePort(c.RemoteAddr().String())

		log.Debug("new incoming connection with IP", minerIp)

		if !rate_limit.CanDoAction(minerIp, rate_limit.ACTION_CONNECT) {
			log.Warn("miner", minerIp, "connect rate limited")
			c.Close()
			continue
		}

		conn := &Connection{
			Conn: c,
			Id:   util.RandomUint64(),

			CData: NewCData(),
		}
		go s.handleConnection(conn)
	}
}

// Server MUST be locked before calling this
func (s *Server) Kick(id uint64) {
	var connectionsNew = make([]*Connection, 0, len(s.Connections))

	for _, v := range s.Connections {
		if v.Id == id {
			log.Debug("kicking peer with ID", id)

			v.Conn.Close()

			ipAddr := util.RemovePort(v.Conn.RemoteAddr().String())

			rate_limit.Disconnect(ipAddr)
		} else {
			connectionsNew = append(connectionsNew, v)
		}
	}
	s.Connections = connectionsNew
}

// this function locks Server
func (srv *Server) handleConnection(conn *Connection) {
	log.Dev("handling connection with ID", conn.Id)

	srv.Lock()
	defer srv.Unlock()

	ipAddr := util.RemovePort(conn.Conn.RemoteAddr().String())

	if !rate_limit.CanConnect(ipAddr) {
		log.Debug("address", ipAddr, "reached connections per IP limit")
		conn.Conn.Close()
		return
	}

	srv.Connections = append(srv.Connections, conn)
	log.Dev("handling connection")

	srv.NewConnections <- conn
}
