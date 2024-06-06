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
	"encoding/json"
	"math"
	"os"
	"sync"
	"time"
	"xelpool/log"
	"xelpool/util"

	"github.com/xelis-project/xelis-go-sdk/wallet"
)

const STATS_INTERVAL = 15 // Minutes
const NUM_CHART_DATA = (60 * 24 / STATS_INTERVAL)

type LastBlock struct {
	Height    uint64 `json:"height"`
	Timestamp int64  `json:"timestamp"`
	Reward    uint64 `json:"reward"`
	Hash      string `json:"hash"`
}

type StatsShare struct {
	Count  uint32 `json:"count"`
	Wallet string `json:"wall"`
	Diff   uint64 `json:"diff"`
	Time   uint64 `json:"time"`
}

type Statistics struct {
	LastUpdate int64

	PoolHashrate      float64
	PoolHashrateChart []Hr
	HashrateCharts    map[string][]Hr

	Hashes float64

	LastBlock LastBlock

	BlocksFound []FoundInfo
	NumFound    int32

	NetHashrate float64
	Difficulty  float64

	KnownAddresses map[string]KnownAddress

	RecentWithdrawals []Withdrawal

	Workers        uint32 // the current number of miners
	WorkersChart   []uint32
	AddressesChart []uint32

	sync.RWMutex
}

type Withdrawal struct {
	Txid         string `json:"txid"`
	Timestamp    uint64 `json:"time"`
	Destinations []wallet.TransferOut
}

type Hr struct {
	Time     int64   `json:"t"`
	Hashrate float64 `json:"h"`
}

type FoundInfo struct {
	Height uint64  `json:"height"`
	Hash   string  `json:"hash"`
	Effort float32 `json:"effort"` // 1 = 100% effort
	Time   uint64  `json:"time"`   // UNIX timestamp
}

var Stats = Statistics{
	HashrateCharts: make(map[string][]Hr),
	KnownAddresses: make(map[string]KnownAddress, NUM_CHART_DATA),
}

type KnownAddress struct {
	LastShare   float64 `json:"t"`
	AvgHashrate float64 `json:"h"`
}

const OFFLINE_AFTER = 6 * 60

func (k *KnownAddress) GetHashrate() float64 {
	if k.LastShare+OFFLINE_AFTER < util.TimePrecise() {
		k.AvgHashrate = 0
		k.LastShare = util.TimePrecise()
	}
	return k.AvgHashrate
}

func (k *KnownAddress) AddShare(diff float64, time float64) {
	if k.LastShare == 0 {
		k.AvgHashrate = 0
		k.LastShare = time
		return
	}

	if k.LastShare > time-1 {
		k.LastShare = time - 1
	}

	s := time - k.LastShare

	hr := diff / s

	const K = 30 // coefficient of smoothing - higher = smoother

	ahr := k.AvgHashrate

	k.AvgHashrate = math.Round(
		((ahr * (K - 1)) + hr) / K)

	log.Dev("AvgHashrate", ahr, "=>", k.AvgHashrate)
	k.LastShare = time
}

func StatsServer() {
	statsData, err := os.ReadFile("stats.json")
	if err != nil {
		log.Warn(err)
	} else {
		Stats.Lock()

		err := json.Unmarshal(statsData, &Stats)
		if err != nil {
			log.Warn(err)
		}

		Stats.Unlock()
	}

	for {
		time.Sleep(100 * time.Millisecond)

		func() {
			Stats.Lock()
			defer Stats.Unlock()

			if time.Now().Unix()-Stats.LastUpdate < STATS_INTERVAL*60 {
				return
			} else if time.Now().Unix()-Stats.LastUpdate > (STATS_INTERVAL*60)*10 {
				Stats.LastUpdate = time.Now().Unix() - STATS_INTERVAL*60
				return
			}

			log.Info("Updating stats")

			Stats.Cleanup()

			Stats.LastUpdate += STATS_INTERVAL * 60

			var totHr float64 = 0

			if Stats.HashrateCharts == nil {
				Stats.HashrateCharts = make(map[string][]Hr, 20)
			}

			for i := range Stats.KnownAddresses {
				v := Stats.KnownAddresses[i]
				hr := v.GetHashrate()
				totHr += hr

				Stats.HashrateCharts[i] = append(Stats.HashrateCharts[i], Hr{
					Time:     Stats.LastUpdate,
					Hashrate: math.Round(hr),
				})

				for len(Stats.HashrateCharts[i]) > NUM_CHART_DATA {
					Stats.HashrateCharts[i] = Stats.HashrateCharts[i][1:]
				}

				didFind := false
				for _, v := range Stats.HashrateCharts[i] {
					if v.Hashrate != 0 {
						didFind = true
					}
				}
				if !didFind {
					delete(Stats.KnownAddresses, i)
					delete(Stats.HashrateCharts, i)
				}
			}

			Stats.WorkersChart = append(Stats.WorkersChart, Stats.Workers)
			Stats.AddressesChart = append(Stats.AddressesChart, uint32(len(Stats.KnownAddresses)))
			Stats.PoolHashrateChart = append(Stats.PoolHashrateChart, Hr{
				Time:     Stats.LastUpdate,
				Hashrate: Round0(totHr),
			})
			for len(Stats.PoolHashrateChart) > NUM_CHART_DATA {
				Stats.PoolHashrateChart = Stats.PoolHashrateChart[1:]
			}
			for len(Stats.WorkersChart) > NUM_CHART_DATA {
				Stats.WorkersChart = Stats.WorkersChart[1:]
			}
			for len(Stats.AddressesChart) > NUM_CHART_DATA {
				Stats.AddressesChart = Stats.AddressesChart[1:]
			}
		}()
	}
}

// Stats MUST be at least RLocked
func (s *Statistics) GetHashrate(wallet string) float64 {
	kaddr := s.KnownAddresses[wallet]

	return kaddr.GetHashrate()
}

// Updates the Pool Hashrate in stats, and saves the stats.
// Stats must be locked.
func (s *Statistics) Cleanup() {
	kaddr := make(map[string]KnownAddress, len(s.KnownAddresses))

	var totalHr float64 = 0

	// clean up known addresses
	for i, v := range s.KnownAddresses {
		if v.LastShare+3600*24 > util.TimePrecise() { // address is not out of date
			kaddr[i] = v
			totalHr += v.GetHashrate()
		}
	}

	s.KnownAddresses = kaddr

	s.PoolHashrate = math.Round(totalHr)

	data, err := json.Marshal(s)
	if err != nil {
		log.Debug(s)
		log.Err(err)
		return
	}

	// only keep the last 50 blocks found
	for len(s.BlocksFound) > 50 {
		s.BlocksFound = s.BlocksFound[:len(s.BlocksFound)-2]
	}

	// only keep the last 50 withdrawal transactions
	for len(s.RecentWithdrawals) > 50 {
		s.RecentWithdrawals = s.RecentWithdrawals[:len(s.RecentWithdrawals)-2]
	}

	os.WriteFile("stats.json", data, 0o600)
}
