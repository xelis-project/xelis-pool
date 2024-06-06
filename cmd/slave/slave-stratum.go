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
	"crypto/rand"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"
	"xelpool/address"
	"xelpool/cfg"
	"xelpool/config"
	"xelpool/log"
	"xelpool/ratelimit"
	"xelpool/stratum"
	"xelpool/util"
	"xelpool/xatum"
	"xelpool/xatum/server"
	"xelpool/xelisutil"
)

type StratumServer struct {
	Conns []*StratumConn

	sync.RWMutex
}

type StratumConn struct {
	Conn net.Conn

	Alive bool
	IP    string

	LastOutID uint32
	MinerID   [16]byte // the first bytes of extra nonce

	CData server.CData
}

func (s *StratumConn) GetExtraNonce() [32]byte {
	var x [32]byte

	copy(x[0:16], s.MinerID[:])
	copy(x[16:16+8], config.POOL_NONCE[:])

	return x
}

// StratumConn MUST be locked before calling this
func (g *StratumConn) WriteJSON(data any) error {
	bin, err := json.Marshal(data)

	if err != nil {
		return err
	}

	log.Net("stratum >>>", string(bin))

	_, err = g.Conn.Write(append(bin, []byte("\n")...))
	return err
}

func (g *StratumConn) Close() error {
	g.Alive = false
	return g.Conn.Close()
}

func handleStratumConns(s *StratumServer) {
	/*listener, err := tls.Listen("tcp", "0.0.0.0:"+util.FormatUint(cfg.Cfg.Slave.StratumPort),
	&tls.Config{Certificates: []tls.Certificate{
		server.Cert,
	}})*/
	listener, err := net.Listen("tcp", "0.0.0.0:"+util.FormatUint(cfg.Cfg.Slave.StratumPort))
	if err != nil {
		log.Fatal(err)
	}

	// Start the pinger
	go func() {
		for {
			time.Sleep((config.SLAVE_MINER_TIMEOUT - 5) * time.Second)

			s.Lock()
			for _, v := range s.Conns {
				v.CData.Lock()

				v.LastOutID++
				v.WriteJSON(stratum.RequestOut{
					Id:     v.LastOutID,
					Method: "mining.ping",
					Params: nil,
				})

				v.CData.Unlock()
			}
			s.Unlock()

		}
	}()

	// Accept incoming connections and handle them
	for {
		Conn, err := listener.Accept()
		if err != nil {
			log.Warn(err)
			continue
		}

		ip := util.RemovePort(Conn.RemoteAddr().String())
		if !ratelimit.CanDoAction(ip, ratelimit.ACTION_CONNECT) {
			log.Warn("Stratum miner", ip, "connect rate limited")
			Conn.Close()
			continue
		}
		if !ratelimit.CanConnect(ip) {
			log.Warn("Stratum miner", ip, "too many open connections")
			Conn.Close()
			continue
		}

		sConn := &StratumConn{
			Conn:    Conn,
			MinerID: GenerateID(),
		}

		log.Debugf("miner has MinerID %x", sConn.MinerID)

		sConn.Alive = true
		sConn.IP = ip

		s.Conns = append(s.Conns, sConn)

		// Handle the connection in a new goroutine
		go handleStratumConn(s, sConn)
	}
}

func GenerateID() [16]byte {
	id := make([]byte, 16)
	rand.Read(id)
	return [16]byte(id)
}

// TODO: put mutexes on disconnect errors
func handleStratumConn(_ *StratumServer, c *StratumConn) {
	rdr := bufio.NewReader(c.Conn)

	// go sendPingPackets(s, Conn)

	numMessages := 0

	for {
		c.CData.Lock()
		var err error
		if numMessages < 2 {
			err = c.Conn.SetReadDeadline(time.Now().Add(config.TIMEOUT * time.Second))
		} else {
			err = c.Conn.SetReadDeadline(time.Now().Add(config.SLAVE_MINER_TIMEOUT * time.Second))
		}
		c.CData.Unlock()
		numMessages++

		if err != nil {
			log.Err("IP", c.IP, "error", err)
			c.Conn.Close()
			c.Alive = false
			return
		}

		str, err := rdr.ReadString('\n')
		if err != nil {
			log.Err("IP", c.IP, "error", err)
			c.Conn.Close()
			c.Alive = false
			return
		}

		log.Net("stratum <<<", str)

		req := stratum.RequestIn{}

		err = json.Unmarshal([]byte(str), &req)
		if err != nil {
			log.Warn(err)
			c.Close()
			c.Alive = false
			return
		}

		switch req.Method {
		case "mining.subscribe":
			MutLastJob.RLock()
			job := LastKnownJob
			MutLastJob.RUnlock()

			// set pool nonce to the POOL_NONCE constant
			job.Blob.SetExtraNonce([32]byte{})
			job.Blob.SetPoolNonce(config.POOL_NONCE)
			job.Blob.SetJobID(c.MinerID)

			xnonce := job.Blob.GetExtraNonce()
			pubkey := job.Blob.GetPublickey()

			c.CData.Lock()
			err := c.WriteJSON(stratum.ResponseOut{
				Id: req.Id,
				Result: []any{
					"",                            // useless (session id)
					hex.EncodeToString(xnonce[:]), // extra nonce
					32,                            // useless (extra nonce length)
					hex.EncodeToString(pubkey[:]), // public key
				},
			})
			if err != nil {
				log.Warn(err)
				c.Alive = false
				c.CData.Unlock()
				return
			}
			c.CData.Unlock()
		case "mining.authorize":
			params := []string{}

			err := json.Unmarshal(req.Params, &params)
			if err != nil {
				log.Warn(err)
				c.Close()
				c.Alive = false
				return
			}

			if len(params) < 3 {
				log.Warn("less than 3 params")
				c.Close()
				c.Alive = false
				return
			}

			params[0] = strings.ReplaceAll(params[0], ".", "+")

			splAddr := strings.Split(params[0], "+")

			var wall string
			if len(splAddr) > 0 {
				wall = splAddr[0]
			}
			var diff uint64 = cfg.Cfg.Slave.InitialDifficulty

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
					diff = diffNum
				}
			}

			if !address.IsAddressValid(wall) {
				c.CData.Lock()

				c.WriteJSON(stratum.ResponseOut{
					Id:     req.Id,
					Result: false,
					Error: &stratum.Error{
						Code:    -1,
						Message: "invalid wallet address",
					},
				})

				log.Warn("Invalid wallet address", wall)
				c.Close()

				c.CData.Unlock()
				return
			}

			log.Info("Stratum miner with address", wall, "IP", c.IP, "connected")

			c.CData.NextDiff = float64(diff)

			c.CData.Lock()
			c.CData.Wallet = wall

			// send the job
			MutLastJob.RLock()
			job := LastKnownJob
			MutLastJob.RUnlock()

			c.CData.Jobs = append(c.CData.Jobs, server.ConnJob{
				Diff: diff,
			})

			// first, send response
			err = c.WriteJSON(stratum.ResponseOut{
				Id:     req.Id,
				Result: true,
			})
			if err != nil {
				log.Warn("failed to send response")
				c.Close()
				c.CData.Unlock()
				return
			}

			// send actual job
			SendStratumJob(c, job.Diff, job.Blob)

			c.CData.Unlock()

		case "mining.submit":
			params := []string{}

			err := json.Unmarshal(req.Params, &params)
			if err != nil {
				log.Warn(err)
				c.Close()
				c.Alive = false
				return
			}

			if len(params) != 3 {
				log.Warn("params length is not 3")
				c.Close()
				c.Alive = false
				return
			}

			jobid, err := hex.DecodeString(params[1])
			if err != nil {
				log.Warn(err)
				c.Close()
				c.Alive = false
				return
			}
			nonceBin, err := hex.DecodeString(params[2])

			if err != nil {
				log.Warn(err)
				c.Close()
				c.Alive = false
				return
			}

			if len(jobid) != 16 || len(nonceBin) != 8 {
				log.Warnf("jobid %x nonce %x do not match expected length (16, 8)", jobid, nonceBin)
				c.Close()
				c.Alive = false
				return
			}

			c.CData.Lock()

			var bm xelisutil.BlockMiner

			for _, v := range c.CData.Jobs {
				if v.BlockMiner.GetJobID() == [16]byte(jobid) {
					bm = v.BlockMiner
					bm.SetNonce(binary.BigEndian.Uint64(nonceBin))
					bm.SetExtraNonce([32]byte{})
					bm.SetPoolNonce(config.POOL_NONCE)
					bm.SetJobID([16]byte(jobid))
				}
			}

			if bm.GetTimestamp() == 0 {
				log.Warnf("outdated share, job id %x", bm.GetJobID())

				c.WriteJSON(stratum.ResponseOut{
					Id:     req.Id,
					Result: false,
					Error: &stratum.Error{
						Code:    21,
						Message: "stale job",
					},
				})
				continue
			}

			jobToSend := &JobToSend{}

			pStr, err := xatum.NewPacket(xatum.PacketC2S_Submit, xatum.C2S_Submit{
				Data: bm[:],
			}).ToString()
			if err != nil {
				log.Err(err)
				c.Close()
				c.CData.Unlock()
				return
			}

			c.CData.Unlock()

			print, shouldKick, err := handleConnPacket(&c.CData, pStr, 10, c.IP, jobToSend, c.MinerID)
			if err != nil {
				log.Err(err)
			}

			if shouldKick {
				c.CData.Lock()
				c.Close()
				c.Alive = false
				c.CData.Unlock()
				return
			}

			if print.Lvl == 3 {
				c.CData.Lock()
				c.WriteJSON(stratum.ResponseOut{
					Id:     req.Id,
					Result: false,
					Error: &stratum.Error{
						Code:    20,
						Message: print.Msg,
					},
				})
				c.CData.Unlock()
			} else {
				c.CData.Lock()
				c.WriteJSON(stratum.ResponseOut{
					Id:     req.Id,
					Result: true,
				})
				c.CData.Unlock()
			}

			if jobToSend.Diff != 0 {
				SendStratumJob(c, jobToSend.Diff, jobToSend.BM)
			}

		default:
			if req.Method != "mining.pong" {
				log.Warn("Unknown Stratum method", req.Method)
			}
		}

	}

}

// NOTE: StratumConn MUST be locked before calling this
func (c *StratumConn) SendDifficulty(diff uint64) error {
	c.LastOutID++

	return c.WriteJSON(stratum.RequestOut{
		Id:     c.LastOutID,
		Method: "mining.set_difficulty",
		Params: []uint64{diff},
	})
}

func (c *StratumConn) SendJob(bm xelisutil.BlockMiner) error {
	c.LastOutID++

	jobid := bm.GetJobID()
	workhash := bm.GetWorkhash()

	timeStr := strconv.FormatUint(bm.GetTimestamp(), 16)

	/*xnonce := c.GetExtraNonce()
	c.WriteJSON(stratum.RequestOut{
		Id:     c.LastOutID,
		Method: "mining.set_extranonce",
		Params: []any{
			hex.EncodeToString(xnonce[:]), // extra nonce
			32,                            // useless parameter
		},
	})
	c.LastOutID++*/

	return c.WriteJSON(stratum.RequestOut{
		Id:     c.LastOutID,
		Method: "mining.notify",
		Params: []any{
			hex.EncodeToString(jobid[:]),
			timeStr,
			hex.EncodeToString(workhash[:]),
			"xel/0",
			true,
		},
	})
}

func SendStratumJob(v *StratumConn, blockDiff uint64, blob xelisutil.BlockMiner) {
	log.Debug("SendJob to Stratum miner with IP", v.Conn.RemoteAddr().String())

	diff := uint64(v.CData.GetNextDiff())

	if diff < cfg.Cfg.Slave.MinDifficulty {
		diff = cfg.Cfg.Slave.MinDifficulty
	} else if diff > blockDiff {
		diff = blockDiff
	}

	jobId := make([]byte, 16)
	_, err := rand.Read(jobId)
	if err != nil {
		log.Err(err)
		return
	}

	// set empty extra nonce
	blob.SetExtraNonce([32]byte{})

	// set pool nonce to the POOL_NONCE constant
	blob.SetPoolNonce(config.POOL_NONCE)

	// set job id to jobId
	blob.SetJobID([16]byte(jobId))

	v.CData.Jobs = append(v.CData.Jobs, server.ConnJob{
		Diff:            diff,
		ChainDiff:       blockDiff,
		BlockMiner:      blob,
		SubmittedNonces: make([]uint64, 0, 8),
	})

	if len(v.CData.Jobs) > config.MAX_PAST_JOBS {
		v.CData.Jobs = v.CData.Jobs[1:]
	}

	log.Devf("sending job to Stratum miner with IP %s (job id %x) ok", v.Conn.RemoteAddr().String(),
		jobId)

	v.SendDifficulty(diff)
	v.SendJob(blob)
}

// sends a job to all the websockets, and removes old websockets
func (s *StratumServer) sendJobs(diff uint64, blob xelisutil.BlockMiner) {
	s.Lock()
	log.Dev("StratumServer sendJobs: num sockets:", len(s.Conns))

	// remove disconnected sockets

	sockets2 := make([]*StratumConn, 0, len(s.Conns))
	for _, c := range s.Conns {
		if c == nil {
			log.Err("THIS SHOULD NOT HAPPEN - connection is nil")
			continue
		}
		if !c.Alive {
			log.Debug("connection with IP", c.IP, "disconnected")

			// DDoS protection disconnect
			ratelimit.Disconnect(c.IP)

			continue
		}
		sockets2 = append(sockets2, c)
	}
	log.Dev("StratumServer sendJobs: going from", len(s.Conns), "to", len(sockets2), "Stratum miners")
	s.Conns = sockets2

	if len(s.Conns) > 0 {
		log.Info("Sending job to", len(s.Conns), "Stratum miners")
	}
	s.Unlock()

	// send jobs to the remaining sockets

	for _, cx := range sockets2 {
		if cx == nil {
			log.Dev("cx is nil")
			continue
		}

		c := cx

		// send job in a new thread to avoid blocking the main thread and reduce latency
		go func() {
			log.Debug("StratumServer sendJobs: sending to IP", c.IP)

			c.CData.Lock()
			defer c.CData.Unlock()

			SendStratumJob(c, diff, blob)

			log.Debug("StratumServer sendJobs: done, sent to IP", c.IP)
		}()
	}
}
