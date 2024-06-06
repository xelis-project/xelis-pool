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
	"fmt"
	"math"
	"sort"
	"strconv"
	"sync"
	"time"
	"xelpool/cfg"
	"xelpool/config"
	"xelpool/database"
	"xelpool/log"
	"xelpool/util"

	"github.com/xelis-project/xelis-go-sdk/daemon"
	"github.com/xelis-project/xelis-go-sdk/wallet"
	bolt "go.etcd.io/bbolt"
)

const SAFETY_MARGIN = 5

// Max number of times that Withdraw() is called
const MAX_WITHDRAW_ATTEMPTS = 10

func Updater() {
	go func() {
		for {
			go func() {
				log.Info("starting Withdraw() loop")
				for i := 0; i < MAX_WITHDRAW_ATTEMPTS; i++ {
					// withdraw funds
					stillUnpaid := Withdraw()

					if !stillUnpaid {
						break
					}
				}
				log.Info("Withdraw() loop done")
			}()
			time.Sleep(time.Duration(config.WITHDRAW_INTERVAL) * time.Second)
		}
	}()

	for {
		time.Sleep(5 * time.Second)

		drpc := newDaemonRPC()

		info, err := drpc.GetInfo()
		if err != nil {
			log.Warn(err)
			continue
		}

		MasterInfo.Lock()
		if info.Topoheight != MasterInfo.Height {
			log.Info("New height", MasterInfo.Height, "->", info.Topoheight)
			MasterInfo.Height = info.Topoheight

			diff, err := strconv.ParseFloat(info.Difficulty, 64)

			if err != nil {
				log.Warn(err)
				continue
			}

			if diff == 0 {
				Stats.NetHashrate = 0
			} else {
				// net hashrate smoothing

				nextHr := diff / float64(cfg.Cfg.BlockTime)

				if nextHr/2 > Stats.NetHashrate || nextHr*2 < Stats.NetHashrate {
					Stats.NetHashrate = nextHr
				} else {
					Stats.NetHashrate = (Stats.NetHashrate*4 + nextHr) / 5
				}
			}
			Stats.Difficulty = Stats.NetHashrate * float64(cfg.Cfg.BlockTime)

			if math.IsInf(Stats.NetHashrate, 0) {
				Stats.NetHashrate = 0
			}

			MasterInfo.Unlock()

			go func() {
				// find new rewards
				UpdatePendingBals()

				// add confirmed rewards
				updated := CheckWithdraw()
				if updated {
					log.Info("CheckWithdraw(): some balances have been updated")
				} else {
					log.Debug("CheckWithdraw(): no balances have been updated")
				}
			}()

		} else {
			MasterInfo.Unlock()
		}

		go UpdateReward()
	}
}

func UpdateReward() {
	drpc := newDaemonRPC()
	info, err := drpc.GetTopBlock(daemon.GetTopBlockParams{
		IncludeTxs: false,
	})
	if err != nil {
		log.Warn(err)
		return
	}

	MasterInfo.Lock()
	defer MasterInfo.Unlock()

	if info.MinerReward == nil {
		log.Warn("miner reward is nil")
		return
	}

	MasterInfo.BlockReward = *info.MinerReward
}

var minHeight uint64
var minHeightMut sync.RWMutex

// adds new pending balances
func UpdatePendingBals() {
	log.Debug("Updating user balances")

	curAddy := cfg.Cfg.PoolAddress

	minHeightMut.Lock()
	MasterInfo.Lock()
	if minHeight == 0 && MasterInfo.Height > SAFETY_MARGIN {
		minHeight = MasterInfo.Height - SAFETY_MARGIN
	}
	MasterInfo.Unlock()

	wrpc := newWalletRPC()

	transfers, err := wrpc.ListTransactions(wallet.ListTransactionsParams{
		AcceptCoinbase: true,
		AcceptBurn:     false,
		AcceptIncoming: false,
		AcceptOutgoing: false,

		MinTopoheight: &minHeight,

		Address: &curAddy,
	})

	minHeightMut.Unlock()
	if err != nil {
		log.Warn(err)
		return
	}

	// Sort transfers by ascending height
	sort.SliceStable(transfers, func(i, j int) bool {
		return transfers[i].Topoheight < transfers[j].Topoheight
	})

	log.Dev("sorted transfers", util.DumpJson(transfers))

	err = DB.Update(func(tx *bolt.Tx) error {
		pendingBuck := tx.Bucket(database.PENDING)
		pendingData := pendingBuck.Get([]byte("pending"))

		pending := database.PendingBals{
			UnconfirmedTxs: make([]database.UnconfTx, 0, 10),
		}

		if pendingData == nil {
			log.Debug("pending is nil")
		} else {
			err := pending.Deserialize(pendingData)
			if err != nil {
				log.Err(err)
				return err
			}
		}

		minHeightMut.Lock()
		if pending.LastHeight > SAFETY_MARGIN {
			minHeight = pending.LastHeight - SAFETY_MARGIN
		}
		minHeightMut.Unlock()

		var totalPendings = make(map[string]uint64)

		nextHeight := pending.LastHeight

		for _, vt := range transfers {
			if vt.Topoheight > pending.LastHeight {
				log.DEBUG("transfer is fine! adding unconfirmed balance to it")

				rewardNoFee := float64(vt.Coinbase.Reward)
				log.Debug("reward before fee is", rewardNoFee/Coin)
				reward := rewardNoFee * (100 - cfg.Cfg.Master.FeePercent) / 100
				log.Debug("reward after fee is", reward/Coin)

				var totHashes float64
				var minersTotalHashes = make(map[string]float64, 10)

				buck := tx.Bucket(database.SHARES)

				c := buck.Cursor()

				for key, v := c.First(); key != nil; key, v = c.Next() {
					sh := database.Share{}

					err := sh.Deserialize(v)

					if err != nil {
						log.Err("error reading share:", err)
						continue
					}

					Stats.RLock()
					// delete outdated shares
					if sh.Time+GetPplnsWindow() < util.Time() {
						Stats.RUnlock()
						buck.Delete(key)
						continue
					}
					Stats.RUnlock()

					totHashes += float64(sh.Diff)
					minersTotalHashes[sh.Wallet] += float64(sh.Diff)

					// log.Dev("block payout: adding share", sh.Wallet, ",", sh.Diff)
				}

				log.Dev("transaction entry", vt)

				txHashBin, err := hex.DecodeString(vt.Hash)
				if err != nil {
					log.Err(err)
					return err
				}
				if len(txHashBin) != 32 {
					log.Err("transaction hash length is not 32 bytes!", vt.Hash)
					return fmt.Errorf("tx hash length is not 32 bytes")
				}

				pendBals := database.UnconfTx{
					UnlockHeight: vt.Topoheight + cfg.Cfg.Master.MinConfs,
					Bals:         make(map[string]uint64),
					TxnBlockHash: [32]byte(txHashBin),
				}

				var minersBalances = make(map[string]float64, len(minersTotalHashes))
				var totalRewarded uint64 // totalReward is slightly smaller than rewardNoFee because of rounding errors
				for i, v := range minersTotalHashes {
					minersBalances[i] = v * float64(reward) / totHashes

					x := uint64(v * float64(reward) / totHashes)
					pendBals.Bals[i] = x

					totalPendings[i] += x

					totalRewarded += x
				}
				pendBals.Bals[cfg.Cfg.FeeAddress] += uint64(rewardNoFee) - totalRewarded

				totalPendings[cfg.Cfg.FeeAddress] += pendBals.Bals[cfg.Cfg.FeeAddress]

				log.Debug("Fee wallet has earned", (rewardNoFee-float64(totalRewarded))/math.Pow10(cfg.Cfg.Atomic))

				log.Dev("total hashes", util.DumpJson(minersTotalHashes))
				log.Dev("balances", util.DumpJson(minersBalances))

				if pending.UnconfirmedTxs == nil {
					pending.UnconfirmedTxs = make([]database.UnconfTx, 0, 10)
				}

				pending.UnconfirmedTxs = append(pending.UnconfirmedTxs, pendBals)

				if vt.Topoheight > nextHeight {
					MasterInfo.RLock()
					nextHeight = MasterInfo.Height
					MasterInfo.RUnlock()
				}

				log.Dev("Adding transfer DONE")
			} else {
				log.Dev("transfer is too old - gotta ignore it - pending.LastHeight is", pending.LastHeight, "txn Height is", vt.Topoheight)
			}
		}

		pending.LastHeight = nextHeight

		infoBuck := tx.Bucket(database.ADDRESS_INFO)

		for i, v := range totalPendings {
			log.Devf("Setting pending %s to %f", i, float64(v)/math.Pow10(cfg.Cfg.Atomic))

			addrInfoBin := infoBuck.Get([]byte(i))

			addrInfo := database.AddrInfo{}

			if addrInfoBin == nil {
				log.Dev("addrInfo is nil")
			} else {
				err = addrInfo.Deserialize(addrInfoBin)
				if err != nil {
					log.Warn(err)
					continue
				}
			}

			addrInfo.BalancePending = v

			err := infoBuck.Put([]byte(i), addrInfo.Serialize())
			if err != nil {
				log.Err(err)
			}
		}

		return pendingBuck.Put([]byte("pending"), pending.Serialize())

	})

	if err != nil {
		log.Err(err)
	}
}
