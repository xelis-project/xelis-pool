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
	"errors"
	"fmt"
	"strconv"
	"sync"
	"time"
	"xelis-pool/cfg"
	"xelis-pool/config"
	"xelis-pool/log"
	"xelis-pool/pow"
	"xelis-pool/rate_limit"
	"xelis-pool/xatum"
	"xelis-pool/xatum/server"

	"github.com/xelis-project/xelis-go-sdk/getwork"
)

func handleDaemon(srv *server.Server, srvgw *GetworkServer, srvstr *StratumServer) {
	log.Debug("handleDaemon")
	for {
		getworkConn(srv, srvgw, srvstr)
	}
}

// MemJob is a fast & efficient struct used for storing a job in memory
type MemJob struct {
	Blob      pow.BlockMiner
	Diff      uint64
	Height    uint64
	Algorithm string
}

var gwConn *getwork.Getwork
var gwMut sync.RWMutex

var LastKnownJob MemJob
var MutLastJob sync.RWMutex

func SubmitBlock(hexData string) error {
	gwMut.Lock()
	defer gwMut.Unlock()

	if gwConn == nil {
		return errors.New("getwork connection is nil")
	}

	return gwConn.SubmitBlock(hexData)
}

func getworkConn(srv *server.Server, srvgw *GetworkServer, srvstr *StratumServer) {
	log.Debug("getworkConn")

	gw, err := getwork.NewGetwork("ws://"+cfg.Cfg.Master.DaemonRpc+"/getwork", cfg.Cfg.PoolAddress, "xelis-pool")
	if err != nil {
		log.Err(err)
		time.Sleep(time.Second)
		return
	}

	gwMut.Lock()
	gwConn = gw
	gwMut.Unlock()

	log.Debug("getwork connected!")

	go getworkAccepts(gw)
	go getworkRejects(gw)

	for {
		job, ok := <-gw.Job
		if !ok {
			log.Err("getwork invalid job received")
			return
		}

		blob, err := hex.DecodeString(job.Template)
		if err != nil {
			log.Warnf("%v", err)
			continue
		}

		job.Algorithm, err = pow.ConvertAlgorithmToStratum(job.Algorithm)
		if err != nil {
			err := fmt.Errorf("unknown algorithm received: %s", job.Algorithm)
			log.Fatal(err)
		}

		log.Infof("new job: height %d blob %s diff %s algo %s", job.Height, job.Template, job.Difficulty,
			job.Algorithm)

		if len(blob) != pow.BLOCKMINER_LENGTH {
			log.Warnf("blob is not %d bytes long", pow.BLOCKMINER_LENGTH)
			continue
		}
		diff, err := strconv.ParseUint(job.Difficulty, 10, 64)
		if err != nil {
			log.Warnf("%v", err)
			continue
		}

		bl := pow.BlockMiner(blob)

		go sendJobs(srv, diff, bl)
		go srvgw.sendGetworkJobs(diff, bl, job.Algorithm)
		go srvstr.sendJobs(diff, bl)

		MutLastJob.Lock()
		LastKnownJob = MemJob{
			Blob:      bl,
			Diff:      diff,
			Height:    job.Height,
			Algorithm: job.Algorithm,
		}
		MutLastJob.Unlock()
	}
}

// NOTE: Connection MUST be locked before calling this
func SendJob(v *server.Connection, blockDiff uint64, blob pow.BlockMiner) {
	log.Debug("SendJob to miner with IP", v.Conn.RemoteAddr().String())

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

	log.Devf("sending job to miner with ID %d IP %s (extra nonce %x) ok", v.Id, v.Conn.RemoteAddr().String(), extraNonce)
	v.SendJob(xatum.S2C_Job{
		Diff: diff,
		Blob: blob[:],
	})
}

func sendJobs(srv *server.Server, diff uint64, blob pow.BlockMiner) {
	log.Info("sendJobs: sending jobs to", len(srv.Connections), "peers")

	srv.Lock()
	defer srv.Unlock()

	for _, vx := range srv.Connections {
		v := vx

		go func() {
			log.Debug("sending job to a connection, waiting for unlock")
			v.CData.Lock()
			defer v.CData.Unlock()
			log.Debug("sending job to a connection, unlock success")

			if v.LastJob().Diff == 0 {
				log.Debug("sendJobs: cannot send job to peer", v.Conn.RemoteAddr(), ": no handshake yet")
				return
			}

			// disconnect peer if it didn't send any share recently
			if time.Since(v.CData.LastShare) > 10*time.Minute {
				log.Debug("sendJobs: disconnecting peer after", time.Since(v.CData.LastShare))

				ip := v.Conn.RemoteAddr().String()

				rate_limit.Ban(ip, time.Now().Unix()+(5*60))

				v.Send(xatum.PacketS2C_Print, xatum.S2C_Print{
					Msg: "no recent share received",
					Lvl: 3,
				})

				srv.Lock()
				srv.Kick(v.Id)
				srv.Unlock()

				return
			}

			log.Dev("sendJobs: sending job to peer", v.Conn.RemoteAddr())

			SendJob(v, diff, blob)
			log.Dev("sendJobs: sent job to peer", v.Conn.RemoteAddr())

		}()

	}
}

func getworkAccepts(gw *getwork.Getwork) {
	for {
		accBl, ok := <-gw.AcceptedBlock
		if !ok {
			log.Warn("getworkAccept closed")
			return
		}
		log.Infof("block accepted: %v", accBl)
	}

}
func getworkRejects(gw *getwork.Getwork) {
	for {
		rejBl, ok := <-gw.RejectedBlock
		if !ok {
			log.Warn("getworkReject closed")
			return
		}
		log.Errf("block rejected: %v", rejBl)
	}

}

/*func setupRPC() (*daemon.RPC, context.Context) {
	ctx := context.Background()
	dae, err := daemon.NewRPC(ctx, "http://"+config.DAEMON_ADDRESS+"/json_rpc")
	if err != nil {
		panic(err)
	}

	return dae, ctx
}*/
