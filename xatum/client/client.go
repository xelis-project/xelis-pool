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

package client

import (
	"bufio"
	"crypto/tls"
	"encoding/json"
	"net"
	"strings"
	"sync"
	"xelpool/log"
	"xelpool/xatum"
)

type Client struct {
	PoolAddress string
	conn        net.Conn

	Jobs    chan xatum.S2C_Job
	Prints  chan xatum.S2C_Print
	Success chan xatum.S2C_Success

	sync.RWMutex
}

func NewClient(poolAddr string) (*Client, error) {
	cl := &Client{
		PoolAddress: poolAddr,

		Jobs:    make(chan xatum.S2C_Job, 1),
		Prints:  make(chan xatum.S2C_Print, 1),
		Success: make(chan xatum.S2C_Success, 1),
	}

	conn, err := tls.Dial("tcp", cl.PoolAddress, &tls.Config{
		InsecureSkipVerify: true, // accept self-signed certificates
	})
	cl.conn = conn
	if err != nil {
		log.Warnf("connection failed: %s", err)
		return nil, err
	}

	return cl, nil
}

func (cl *Client) Connect() {
	rdr := bufio.NewReader(cl.conn)
	for {
		str, err := rdr.ReadString('\n')

		if err != nil {
			cl.conn.Close()
			log.Warnf("connection closed: %s", err)
			return
		}
		log.Net("<<<", str)

		spl := strings.SplitN(str, "~", 2)
		if len(spl) < 2 {
			log.Warn("packet data is malformed")
			continue
		}

		pack := spl[0]

		switch pack {
		case xatum.PacketS2C_Job:
			pData := xatum.S2C_Job{}

			err := json.Unmarshal([]byte(spl[1]), &pData)
			if err != nil {
				log.Warn("failed to parse data")
				cl.conn.Close()
				return
			}

			log.Debug("ok, job received, sending to channel")

			cl.Jobs <- pData

			log.Debug("ok, done sending to channel")
		case xatum.PacketS2C_Print:
			pData := xatum.S2C_Print{}
			err := json.Unmarshal([]byte(spl[1]), &pData)
			if err != nil {
				log.Warn("failed to parse data")
				cl.conn.Close()
				return
			}

			const PREFIX = "message from pool:"

			switch pData.Lvl {
			case 1:
				log.Infof(PREFIX+" %s", pData.Msg)
			case 2:
				log.Warnf(PREFIX+" %s", pData.Msg)
			case 3:
				log.Errf(PREFIX+" %s", pData.Msg)
			}

		case xatum.PacketS2C_Ping:
			cl.Send("pong", map[string]any{})
		default:
			log.Warnf("Unknown packet %s", pack)
		}

	}

}

// Client MUST be locked before calling this
func (c *Client) Send(name string, a any) error {
	data, err := json.Marshal(a)
	if err != nil {
		panic(err)
	}
	c.SendBytes(append([]byte(name+"~"), data...))
	return nil
}

// Client MUST be locked before calling this
func (cl *Client) SendBytes(data []byte) error {
	_, err := cl.conn.Write(append(data, '\n'))
	if err != nil {
		return err
	}

	return nil
}

func (cl *Client) Submit(pack xatum.C2S_Submit) error {

	return cl.Send(xatum.PacketC2S_Submit, pack)
}
