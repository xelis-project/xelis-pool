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
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"
	"xelpool/address"
	"xelpool/cfg"
	"xelpool/config"
	"xelpool/log"
	"xelpool/ratelimit"
	"xelpool/slave"
	"xelpool/util"
	"xelpool/xatum"
	"xelpool/xatum/server"
	"xelpool/xelisutil"
)

func GenExtraNonce() (x [32]byte) {
	minerId := make([]byte, 16)
	rand.Read(minerId)

	copy(x[0:16], minerId)
	copy(x[16:16+8], config.POOL_NONCE[:])

	return
}

// GENERIC SLAVE METHODS

type JobToSend struct {
	Diff uint64
	BM   xelisutil.BlockMiner
}

func handleConnPacket(cdat *server.CData, str string, packetsRecv int, ip string, toSend *JobToSend, minerId [16]byte) (*xatum.S2C_Print, bool, error) {
	spl := strings.SplitN(str, "~", 2)
	if spl == nil || len(spl) < 2 {
		log.Warn("packet data is malformed, spl:", spl)
		return &xatum.S2C_Print{
			Msg: "malformed packet data",
			Lvl: 3,
		}, true, nil
	}

	pack := spl[0]

	if packetsRecv == 1 && spl[0] != xatum.PacketC2S_Handshake {
		return &xatum.S2C_Print{
			Msg: "first packet must be a handshake",
			Lvl: 3,
		}, true, errors.New("first packet must be a handshake")
	}

	if pack == xatum.PacketC2S_Handshake {
		if packetsRecv != 1 {
			return &xatum.S2C_Print{
				Msg: "more than one handshake received",
				Lvl: 3,
			}, true, errors.New("more than one handshake received")
		}

		pData := xatum.C2S_Handshake{}

		err := json.Unmarshal([]byte(spl[1]), &pData)
		if err != nil {
			return &xatum.S2C_Print{
				Msg: "failed to parse data",
				Lvl: 3,
			}, true, errors.New("failed to parse data")
		}

		pData.Addr = strings.ReplaceAll(pData.Addr, ".", "+")

		splAddr := strings.Split(pData.Addr, "+")

		var wall string
		if len(splAddr) > 0 {
			wall = splAddr[0]
		}

		if !address.IsAddressValid(wall) {
			return &xatum.S2C_Print{
				Msg: "invalid address",
				Lvl: 3,
			}, true, errors.New("IP " + ip + " invalid address " + wall)
		}

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
				cdat.Lock()
				cdat.NextDiff = float64(diffNum)
				cdat.Unlock()
			}
		}

		if !slices.Contains(pData.Algos, "xel/0") && !slices.Contains(pData.Algos, "xel/1") {
			return &xatum.S2C_Print{
				Msg: "your miner does not support xel/0 or xel/1 algorithms",
				Lvl: 3,
			}, true, errors.New("your miner does not support xel/0 or xel/1 algorithms")
		}

		log.Infof("New miner | Address: %s %s UserAgent: %s Algos: %s", wall, pData.Work, pData.Agent, pData.Algos)

		cdat.Lock()
		cdat.Wallet = wall
		cdat.Unlock()

		// send first job

		MutLastJob.Lock()

		diff := LastKnownJob.Diff
		blob := LastKnownJob.Blob

		MutLastJob.Unlock()

		log.Debugf("first job diff %d blob %x", diff, blob)

		toSend.Diff = diff
		toSend.BM = blob

	} else if pack == xatum.PacketC2S_Pong {
		log.Dev("received pong packet")
	} else if pack == xatum.PacketC2S_Submit {

		ipAddr := util.RemovePort(ip)

		if !ratelimit.CanDoAction(ipAddr, ratelimit.ACTION_SHARE_SUBMIT) {
			return &xatum.S2C_Print{
				Msg: "too many submit packets",
				Lvl: 3,
			}, true, fmt.Errorf("IP %s submit packet rate-limited", ipAddr)
		}

		pData := xatum.C2S_Submit{}

		err := json.Unmarshal([]byte(spl[1]), &pData)
		if err != nil {
			return &xatum.S2C_Print{
				Msg: "too many submit packets",
				Lvl: 3,
			}, true, fmt.Errorf("failed to parse data")
		}

		// validate PoW hash
		ForcePowCheck := false

		powHash, err := hex.DecodeString(pData.Hash)
		if err != nil || len(powHash) != 32 {
			log.Dev("there is no pow hash data, forcing PoW check")
			ForcePowCheck = true
		}

		// validate BlockMiner length
		if len(pData.Data) != xelisutil.BLOCKMINER_LENGTH {
			return &xatum.S2C_Print{
				Msg: "invalid blockminer length",
				Lvl: 3,
			}, true, fmt.Errorf("invalid blockminer length")
		}

		bm := xelisutil.BlockMiner(pData.Data)

		// validate extra nonce / job id
		cdat.Lock()
		var minerJob *server.ConnJob

		if bm.GetPoolNonce() != config.POOL_NONCE {
			log.Warnf("user sent invalid pool nonce, expected %x, got %x",
				config.POOL_NONCE, bm.GetPoolNonce())

			return &xatum.S2C_Print{
				Msg: "invalid pool nonce",
				Lvl: 3,
			}, true, fmt.Errorf("invalid pool nonce")
		}

		jobid := bm.GetJobID()

		if jobid == [16]byte{} {
			log.Warn("user sent 00 job id in extra nonce, which is not valid")

			return &xatum.S2C_Print{
				Msg: "blank extra nonce",
				Lvl: 3,
			}, true, fmt.Errorf("blank extra nonce")
		}

		for i, v := range cdat.Jobs {
			if jobid == v.BlockMiner.GetJobID() {
				log.Debugf("share uses job with age %d, jobid %x", len(cdat.Jobs)-1-i, jobid)
				minerJob = &cdat.Jobs[i]
			}
		}
		if minerJob == nil {
			nonces := make([][32]byte, len(cdat.Jobs))

			if len(cdat.Jobs) > 0 {
				for i, v := range cdat.Jobs {
					nonces[len(cdat.Jobs)-1-i] = v.BlockMiner.GetExtraNonce()
				}
			}

			err = fmt.Errorf("%v: stale share, expected nonce extra %x, got %x",
				ip, nonces, bm.GetExtraNonce())

			cdat.LastShare = time.Now()
			cdat.Unlock()

			return &xatum.S2C_Print{
				Msg: err.Error(),
				Lvl: 3,
			}, false, err
		}
		cdat.Unlock()

		// set JobID to the actual MinerID
		if minerId != [16]byte{} {
			bm.SetJobID(minerId)
		}

		// validate Workhash and Publickey
		if bm.GetWorkhash() != minerJob.BlockMiner.GetWorkhash() ||
			bm.GetPublickey() != minerJob.BlockMiner.GetPublickey() {

			err := fmt.Errorf("invalid Workhash or Publickey, WorkHash %x, got %x; PublicKey %x, got %x",
				minerJob.BlockMiner.GetWorkhash(), bm.GetWorkhash(),
				minerJob.BlockMiner.GetPublickey(), bm.GetPublickey())

			return &xatum.S2C_Print{
				Msg: "share accepted (dev)",
				Lvl: 1,
			}, false, err
		}

		// validate nonce
		if slices.Contains(minerJob.SubmittedNonces, bm.GetNonce()) {
			cdat.Lock()
			defer cdat.Unlock()

			return &xatum.S2C_Print{
				Msg: "duplicate nonce",
				Lvl: 3,
			}, false, errors.New("duplicate nonce")
		}

		minerJob.SubmittedNonces = append(minerJob.SubmittedNonces, bm.GetNonce())

		// validate timestamp
		if bm.GetTimestamp() < minerJob.BlockMiner.GetTimestamp() ||
			bm.GetTimestamp() > uint64(time.Now().UnixMilli()+config.TIMESTAMP_FUTURE_LIMIT*1000) {

			err := fmt.Errorf("timestamp is too much in the past/future: %d, current: %d", bm.GetTimestamp(), time.Now().UnixMilli())

			return &xatum.S2C_Print{
				Msg: "timestamp is too much in the past or future, check that your clock is synchronized",
				Lvl: 3,
			}, true, err
		}

		go func() {
			MutLastJob.RLock()
			algo := LastKnownJob.Algo
			if algo != "xel/0" && algo != "xel/1" {
				log.Err("algo is invalid: " + algo + " - setting it to xel/1")
				algo = "xel/1"
			}
			MutLastJob.RUnlock()

			// validate PoW if forced

			if ForcePowCheck {
				t := time.Now()

				log.Debug("computing Forced PoW with algo", algo)

				pow := bm.PowHash(algo)

				powHash = pow[:]

				log.Debugf("computed Forced PoW in %s, result %x", time.Since(t), pow)
			}

			if [32]byte(powHash) == [32]byte{} {
				log.Errf("invalid blank pow data %x", powHash)
				return
			}

			// validate difficulty
			if !xelisutil.CheckDiff([32]byte(powHash), minerJob.Diff) {
				log.Warn("hash does not meet target, ForcePowCheck:", ForcePowCheck)
				if ForcePowCheck {
					cdat.Lock()
					cdat.Score = -cfg.Cfg.Slave.TrustScore
					cdat.Unlock()
				}

				log.Debug(hex.EncodeToString(bm[:]))
				log.Debug(bm.ToString())

				return
			}

			var findsBlock = xelisutil.CheckDiff([32]byte(powHash), minerJob.ChainDiff)

			// validate PoW when not forced on

			if !ForcePowCheck {
				cdat.RLock()
				score := cdat.Score
				cdat.RUnlock()

				if findsBlock ||
					score < cfg.Cfg.Slave.TrustScore ||
					util.RandomFloat()*100 < cfg.Cfg.Slave.TrustedCheckChance {

					t := time.Now()

					pow := bm.PowHash(algo)

					log.Debugf("PoW checked in %v algo: %s", time.Since(t).String(), algo)

					if pow != [32]byte(powHash) {
						cdat.Lock()
						cdat.Score = -cfg.Cfg.Slave.TrustScore
						cdat.Unlock()

						err := fmt.Errorf("invalid pow hash: %x, expected %x", powHash, pow)
						log.Warn(err)
						return
					}
				} else {
					log.Debugf("skipping share check (trust score %d)", cfg.Cfg.Slave.TrustScore)
				}
			}
			// SHARE IS CONSIDERED VALID
			cdat.Lock()
			cdat.Score++
			deltaT := float64(time.Since(cdat.LastShare).Nanoseconds()) * time.Nanosecond.Seconds()
			hr := float64(minerJob.Diff) / deltaT
			log.Debugf("Hashrate: %.1f H/s", hr)
			cdat.LastShare = time.Now()
			futDiff := hr * cfg.Cfg.Slave.ShareTarget
			if futDiff < float64(cfg.Cfg.Slave.MinDifficulty) {
				futDiff = float64(cfg.Cfg.Slave.MinDifficulty)
			}

			// new connections have faster diff adjustment
			var K float64 = 20
			if cdat.Score >= 0 && cdat.Score < 5 {
				K = 5
			} else if cdat.Score < 15 {
				K = 12
			}

			cdat.NextDiff = (cdat.NextDiff*(K-1) + futDiff) / K
			log.Debug("next diff:", cdat.GetNextDiff())
			cdat.Unlock()

			cdat.RLock()
			slave.SendShare(cdat.Wallet, minerJob.Diff)
			cdat.RUnlock()

			// if share finds a block, submit it
			if findsBlock {
				log.Info("BLOCK FOUND")
				log.Infof("Found block %x", bm.Hash())
				log.Infof("PoW hash %x, diff %d", powHash, minerJob.ChainDiff)
				err = SubmitBlock(hex.EncodeToString(bm[:]))
				if err != nil {
					log.Warnf("failed to submit block: %v", err)

					go func() {
						time.Sleep(5 * time.Second)
						err = SubmitBlock(hex.EncodeToString(bm[:]))
						log.Err("block resubmit attempt:", err)
					}()

					return
				}

				slave.SendBlockFound(bm.Hash())
			}
			// if the difficulty changed too much, send a new job with updated difficulty

			cdat.Lock()
			defer cdat.Unlock()

			if cdat.NextDiff > float64(cdat.LastJob().Diff)*4 ||
				cdat.NextDiff < float64(cdat.LastJob().Diff)*0.5 {

				toSend.Diff = cdat.LastJob().ChainDiff
				toSend.BM = cdat.LastJob().BlockMiner
			}
		}()

		return &xatum.S2C_Print{
			Msg: "share accepted",
			Lvl: 1,
		}, false, nil

	} else {
		err := fmt.Errorf("unknown packet %s", pack)

		return &xatum.S2C_Print{
			Msg: "unknown packet" + pack,
			Lvl: 3,
		}, false, err
	}

	return nil, false, nil
}
