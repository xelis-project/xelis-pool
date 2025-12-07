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
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"xelpool/address"
	"xelpool/cfg"
	"xelpool/config"
	"xelpool/log"
	"xelpool/pow"
	"xelpool/rate_limit"
	"xelpool/xatum"
	"xelpool/xatum/server"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

type GetworkConn struct {
	conn *websocket.Conn

	Alive bool
	IP    string

	CData server.CData
}

// GetworkConn MUST be locked before calling this
func (g *GetworkConn) WriteJSON(data interface{}) error {
	return g.conn.WriteJSON(data)
}

func (g *GetworkConn) Close() error {
	return g.conn.Close()
}

type GetworkServer struct {
	Conns []*GetworkConn

	sync.RWMutex
}

var upgrader = websocket.Upgrader{} // use default options

func fmtMessageType(mt int) string {
	if mt == websocket.BinaryMessage {
		return "binary"
	} else if mt == websocket.TextMessage {
		return "text"
	} else {
		return "Unknown Message Type"
	}
}

// sends a job to all the websockets, and removes old websockets
func (s *GetworkServer) sendGetworkJobs(diff uint64, blob pow.BlockMiner, algo string) {
	sockets2 := make([]*GetworkConn, 0)
	func() {
		s.Lock()
		defer s.Unlock()
		log.Dev("sendJobToWebsocket: num sockets:", len(s.Conns))

		// remove disconnected sockets

		for _, c := range s.Conns {
			if c == nil {
				log.Err("THIS SHOULD NOT HAPPEN - connection is nil")
				continue
			}
			if !c.Alive {
				log.Debug("connection with IP", c.IP, "disconnected")

				// DDoS protection disconnect
				rate_limit.Disconnect(c.IP)

				continue
			}
			sockets2 = append(sockets2, c)
		}
		log.Dev("sendJobToWebsocket: going from", len(s.Conns), "to", len(sockets2), "getwork miners")
		s.Conns = sockets2

		if len(s.Conns) > 0 {
			log.Info("Sending job to", len(s.Conns), "GetWork miners")
		}

	}()

	// send jobs to the remaining sockets

	for _, cx := range sockets2 {
		if cx == nil {
			log.Dev("cx is nil")
			continue
		}

		c := cx

		// send job in a new thread to avoid blocking the main thread and reduce latency
		go func() {
			log.Debug("sendJobToWebsocket: sending to IP", c.IP)

			c.CData.Lock()
			defer c.CData.Unlock()

			err := SendJobGetwork(c, diff, blob, algo)

			// if write failed, close the connection (if it isn't already closed)
			if err != nil {
				log.Warn("sendJobToWebsocket: cannot send job:", err)
				c.Close()
				c.Alive = false
				return
			}

			log.Debug("sendJobToWebsocket: done, sent to IP", c.IP)
		}()
	}
}

func getIPV4list() []string {
	resp, err := http.Get("https://www.cloudflare.com/ips-v4")

	if err != nil {
		log.Fatal(err)
	}

	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)

	if err != nil {
		log.Fatal(err)
	}

	ls := strings.Split(strings.ReplaceAll(string(body), "\r", ""), "\n")

	log.Info("Cloudflare IP addresses:", ls)

	return ls
}

func (s *GetworkServer) listenGetwork() {
	r := gin.Default()

	r.RemoteIPHeaders = append(r.RemoteIPHeaders, "True-Client-IP")
	r.ForwardedByClientIP = true

	ipList := getIPV4list()

	r.SetTrustedProxies(append(ipList, "127.0.0.1"))

	r.GET("/getwork/:addr/*worker", func(c *gin.Context) {
		// DDoS protection
		if !rate_limit.CanConnect(c.ClientIP()) {
			log.Warn("IP", c.ClientIP(), "has too many Getwork connections")
			c.String(429, "429 too many open connections")

			return
		}

		if !rate_limit.CanDoAction(c.ClientIP(), rate_limit.ACTION_CONNECT) {
			log.Warn("IP", c.ClientIP(), "rate limited on Getwork server")
			c.String(429, "429 too many requests")

			return
		}

		addy, hasParam := c.Params.Get("addr")

		if !hasParam || len(addy) < 5 {
			c.String(400, "400 wallet address is missing")

			return
		}

		splAddr := strings.Split(strings.Split(addy, ".")[0], "+")

		var wall string
		if len(splAddr) > 0 {
			wall = splAddr[0]
		}
		var diff float64

		if len(splAddr) > 1 {
			diffStr := splAddr[1]

			diffNum, err := strconv.ParseUint(diffStr, 10, 64)

			if err != nil {
				log.Debug(err)
			} else {
				const MAX = 10_000_000

				if diffNum < cfg.Cfg.Slave.MinDifficulty {
					diffNum = cfg.Cfg.Slave.MinDifficulty
				} else if diffNum > MAX {
					diffNum = MAX
				}
				diff = float64(diffNum)
			}
		}

		worker, _ := c.Params.Get("worker")

		if !address.IsAddressValid(wall) {
			c.String(400, "400 invalid wallet address")

			return
		}

		if worker == "" {
			worker = "x"
		}

		log.Info("new GetWork miner with IP", c.ClientIP(), "wallet", addy, "worker", worker)

		s.Lock()
		gwConn := &GetworkConn{
			Alive: true,
			IP:    c.ClientIP(),
			CData: server.NewCData(),
		}
		gwConn.CData.Wallet = wall
		gwConn.CData.NextDiff = diff
		s.Conns = append(s.Conns, gwConn)
		s.Unlock()

		s.wsHandler(gwConn, c.Writer, c.Request)
	})
	r.Run(":" + strconv.FormatUint(uint64(cfg.Cfg.Slave.GetworkPort), 10))
}

type BlockTemplate struct {
	Difficulty string `json:"difficulty"`
	Height     uint64 `json:"height"`
	TopoHeight uint64 `json:"topoheight"`
	Template   string `json:"template"`
}

func (s *GetworkServer) wsHandler(c *GetworkConn, w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Warn("upgrade:", err)
		return
	}
	defer conn.Close()

	c.conn = conn

	log.Info("Miner with IP", c.IP, "connected to Getwork")

	// send first job
	c.CData.Lock()

	MutLastJob.Lock()
	if LastKnownJob.Diff == 0 {
		log.Debug("not sending first job, because there is no first job yet")
		MutLastJob.Unlock()
		c.CData.Unlock()
		return
	}

	firstJobDiff := LastKnownJob.Diff
	firstJobBlob := LastKnownJob.Blob
	firstJobAlgo := LastKnownJob.Algorithm

	MutLastJob.Unlock()

	err = SendJobGetwork(c, firstJobDiff, firstJobBlob, firstJobAlgo)

	c.CData.Unlock()

	if err != nil {
		log.Warn("failed to send first job:", err)
	}
	// done sending first job

	log.Debug("done sending first job")

	packetsRecv := 1 // getwork starts from packet number 2, since handshake is integrated in HTTP

	for {
		mt, message, err := c.conn.ReadMessage()
		if err != nil {
			log.Info("Getwork miner disconnected:", err)
			break
		}

		packetsRecv++

		log.Debugf("recv: %s, type: %s", message, fmtMessageType(mt))

		var msgJson map[string]any

		err = json.Unmarshal([]byte(message), &msgJson)

		if err != nil {
			log.Err(err)
		}

		if msgJson["miner_work"] == nil {
			if msgJson["block_template"] == nil {
				log.Debug("miner_work and block_template are nil")
				continue
			} else {
				msgJson["miner_work"] = msgJson["block_template"]
			}
		}

		minerWork := msgJson["miner_work"].(string)

		minerBlob, err := hex.DecodeString(minerWork)
		if err != nil {
			log.Err(err)
			continue
		}

		if len(minerBlob) != pow.BLOCKMINER_LENGTH {
			log.Info()
			continue
		}

		var jobToSend JobToSend

		str, err := xatum.NewPacket(xatum.PacketC2S_Submit, xatum.C2S_Submit{
			Data: minerBlob,
		}).ToString()

		if err != nil {
			log.Err(err)
			continue
		}

		log.Dev("str:", str)

		print, shouldKick, err := handleConnPacket(&c.CData, str, packetsRecv, c.IP, &jobToSend, [16]byte{})
		if err != nil {
			log.Warn("Getwork:", err)
		}
		if print != nil && print.Lvl == 1 { // send "accepted" reply
			c.CData.Lock()
			err = c.conn.WriteMessage(websocket.TextMessage, []byte(`"block_accepted"`))
			c.CData.Unlock()

			if err != nil {
				log.Err("failed to send accepted reply:", err)
				shouldKick = true
			}
		} else if print != nil { // send "rejected" reply
			c.CData.Lock()

			err = c.conn.WriteMessage(websocket.TextMessage, []byte(`{"block_rejected":"`+print.Msg+`"}`))
			c.CData.Unlock()

			if err != nil {
				log.Err("failed to send reject reply:", err)
				shouldKick = true
			}
		}

		if shouldKick {
			s.Lock()
			err := c.Close()
			if err != nil {
				log.Debug(err)
			}
			s.Unlock()
		}

		if jobToSend.Diff != 0 {
			log.Debug("jobToSend has diff", jobToSend.Diff)
			SendJobGetwork(c, jobToSend.Diff, jobToSend.BM, firstJobAlgo)
		}
	}
}

// NOTE: Connection MUST be locked before calling this
func SendJobGetwork(v *GetworkConn, blockDiff uint64, blob pow.BlockMiner, algorithm string) error {
	algorithm, err := pow.ConvertAlgorithmToGetwork(algorithm)
	if err != nil {
		return err
	}

	log.Debug("SendJobGetwork blockDiff", blockDiff)

	diff := uint64(v.CData.GetNextDiff())

	if diff > blockDiff {
		diff = blockDiff
	}

	extraNonce := GenExtraNonce()

	blob.SetExtraNonce([32]byte(extraNonce))

	v.CData.Jobs = append(v.CData.Jobs, server.ConnJob{
		Diff:            diff,
		ChainDiff:       blockDiff,
		BlockMiner:      blob,
		SubmittedNonces: make([]uint64, 0, 8),
	})

	if len(v.CData.Jobs) > config.MAX_PAST_JOBS {
		v.CData.Jobs = v.CData.Jobs[1:]
	}

	log.Devf("sending job to Getwork miner with IP %s (extra nonce %x)", v.IP, extraNonce)
	return v.WriteJSON(map[string]any{
		"new_job": map[string]any{
			"difficulty": strconv.FormatUint(diff, 10),
			"height":     0,
			"topoheight": 0,
			"miner_work": hex.EncodeToString(blob[:]),
			"algorithm":  algorithm,
		},
	})
}
