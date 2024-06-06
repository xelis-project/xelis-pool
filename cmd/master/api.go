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
	"fmt"
	"math"
	"slices"
	"strconv"
	"strings"
	"xelpool/cfg"
	"xelpool/config"
	"xelpool/database"
	"xelpool/log"

	"github.com/gin-gonic/gin"
	"github.com/xelis-project/xelis-go-sdk/wallet"
	bolt "go.etcd.io/bbolt"
)

type UserWithdrawal struct {
	Amount float64 `json:"amount"`
	Txid   string  `json:"txid"`
	Time   uint64  `json:"time"`
}

type PubWithdraw struct {
	Txid         string  `json:"txid"`
	Timestamp    uint64  `json:"time"`
	Amount       float64 `json:"amount"`
	Destinations int     `json:"destinations"`
}

var Coin float64

func cors() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		c.Next()
	}
}

func StartApiServer() {
	Coin = math.Pow10(cfg.Cfg.Atomic)

	gin.SetMode("release")
	r := gin.Default()

	r.SetTrustedProxies([]string{
		"127.0.0.1",
	})

	r.Use(cors())

	r.GET("/ping", func(c *gin.Context) {
		c.String(200, "pong")
	})

	r.GET("/stats", func(c *gin.Context) {
		c.Header("Cache-Control", "max-age=10")

		MasterInfo.RLock()
		height := MasterInfo.Height
		reward := MasterInfo.BlockReward
		MasterInfo.RUnlock()

		Stats.RLock()
		defer Stats.RUnlock()

		var ws = make([]PubWithdraw, 0, len(Stats.RecentWithdrawals))

		for _, v := range Stats.RecentWithdrawals {
			var x uint64
			for _, v2 := range v.Destinations {
				x += v2.Amount
			}

			ws = append(ws, PubWithdraw{
				Txid:         v.Txid,
				Timestamp:    v.Timestamp,
				Amount:       Round6(float64(x) / math.Pow10(cfg.Cfg.Atomic)),
				Destinations: len(v.Destinations),
			})
		}

		netHr := Stats.NetHashrate
		if math.IsInf(netHr, 0) {
			netHr = 1
		}

		netDiff := Stats.Difficulty
		if math.IsInf(netDiff, 0) {
			netDiff = 1
		}

		if math.IsInf(Stats.Hashes, 0) {
			Stats.Hashes = 1
		}

		x := gin.H{
			"pool_hr":             Stats.PoolHashrate,
			"net_hr":              netHr / 32, // TODO: this is a hacky fix
			"connected_addresses": len(Stats.KnownAddresses),
			"connected_workers":   Stats.Workers,
			"chart": gin.H{
				"hashrate":  Stats.PoolHashrateChart,
				"workers":   Stats.WorkersChart,
				"addresses": Stats.AddressesChart,
			},
			"num_blocks_found":    Stats.NumFound,
			"recent_blocks_found": Stats.BlocksFound,
			"height":              height,
			"last_block":          Stats.LastBlock,

			"reward": Round3(float64(reward) / Coin),

			"pplns_window_seconds": GetPplnsWindow(),
			"withdrawals":          ws,
			"hashes":               Stats.Hashes,
			"difficulty":           netDiff,
			"effort":               Stats.Hashes / netDiff * 32,

			// stats that do not change

			"pool_fee_percent": cfg.Cfg.Master.FeePercent,
			// "stratums":          cfg.Cfg.Master.Stratums,
			"payment_threshold": cfg.Cfg.Master.MinWithdrawal,
		}

		c.JSON(200, x)
	})

	r.GET("/stats/:addr", func(c *gin.Context) {
		c.Header("Cache-Control", "max-age=10")

		addrSpl := strings.Split(c.Param("addr"), "+")

		if len(addrSpl) == 0 {
			return
		}

		addr := addrSpl[0]

		if (addr == cfg.Cfg.PoolAddress || addr == cfg.Cfg.FeeAddress) &&
			(len(addrSpl) < 2 || addrSpl[1] != "secp256k1") {

			log.Debug("sending address not found for fee address")

			c.JSON(404, gin.H{
				"error": gin.H{
					"code":    1, // address not found
					"message": "address not found",
				},
			})

			return
		}

		addrInfo := database.AddrInfo{}

		DB.View(func(tx *bolt.Tx) error {
			buck := tx.Bucket(database.ADDRESS_INFO)

			addrData := buck.Get([]byte(addr))
			if addrData == nil {
				return fmt.Errorf("unknown address %s", addr)
			}

			return addrInfo.Deserialize(addrData)
		})

		Stats.RLock()
		defer Stats.RUnlock()

		uw := []UserWithdrawal{}

		for _, v := range Stats.RecentWithdrawals {
			for _, v2 := range v.Destinations {
				if v2.Destination == addr {
					uw = append(uw, UserWithdrawal{
						Amount: float64(v2.Amount) / Coin,
						Txid:   v.Txid,
						Time:   v.Timestamp,
					})
				}
			}

		}

		c.JSON(200, gin.H{
			"hashrate":        NotNan(Round0(Stats.GetHashrate(addr))),
			"balance":         NotNan(Round6(float64(addrInfo.Balance) / Coin)),
			"balance_pending": NotNan(Round6(float64(addrInfo.BalancePending) / Coin)),
			"paid":            NotNan(Round6(float64(addrInfo.Paid) / Coin)),
			"est_pending":     NotNan(Round6(GetEstPendingBalance(addr))),
			"hr_chart":        Stats.HashrateCharts[addr],
			"withdrawals":     uw,
		})
	})

	r.GET("/info", func(c *gin.Context) {
		c.Header("Cache-Control", "max-age=3600")
		c.JSON(200, gin.H{
			"pool_fee_percent":  cfg.Cfg.Master.FeePercent,
			"payment_threshold": cfg.Cfg.Master.MinWithdrawal,
		})
	})

	r.GET("/admin/:pass/backup", func(ctx *gin.Context) {
		pass := ctx.Param("pass")
		if pass != cfg.Cfg.MasterPass {
			ctx.String(404, "404")
			return
		}

		err := DB.View(func(tx *bolt.Tx) error {
			ctx.Header("Content-Type", "application/octet-stream")
			ctx.Header("Content-Disposition", `attachment; filename="my.db"`)
			ctx.Header("Content-Length", strconv.Itoa(int(tx.Size())))
			_, err := tx.WriteTo(ctx.Writer)
			return err
		})
		if err != nil {
			ctx.String(500, "internal server error")
		}
	})

	r.GET("/admin/:pass/withdrawals", func(ctx *gin.Context) {
		pass := ctx.Param("pass")
		if pass != cfg.Cfg.MasterPass {
			ctx.String(404, "404")
			return
		}

		Stats.Lock()
		defer Stats.Unlock()

		ctx.JSON(200, Stats.RecentWithdrawals)
	})

	r.GET("/admin/:pass/", func(ctx *gin.Context) {
		pass := ctx.Param("pass")
		if pass != cfg.Cfg.MasterPass {
			ctx.String(404, "404")
			return
		}

		var confirmed uint64 = 0
		var pending uint64 = 0

		DB.View(func(tx *bolt.Tx) error {
			buck := tx.Bucket(database.ADDRESS_INFO)

			err := buck.ForEach(func(k, v []byte) error {
				ai := database.AddrInfo{}
				err := ai.Deserialize(v)

				if err != nil {
					log.Warn(err)
					return err
				}

				confirmed += ai.Balance
				pending += ai.BalancePending

				return nil
			})
			if err != nil {
				log.Err(err)
			}

			return err
		})

		rpc := newWalletRPC()
		balance, err := rpc.GetBalance(wallet.GetBalanceParams{
			Asset: config.ASSET,
		})

		if err != nil {
			log.Warn(err)
		}

		Stats.Lock()
		defer Stats.Unlock()

		var miners = make([]minerLog, 0, len(Stats.KnownAddresses))

		for addr, v := range Stats.KnownAddresses {
			miners = append(miners, minerLog{
				Hashrate: v.GetHashrate(),
				Wallet:   addr,
			})
		}

		slices.SortFunc(miners, func(a, b minerLog) int {
			d := b.Hashrate - a.Hashrate

			if d > 0 {
				return 1
			} else if d < 0 {
				return -1
			}
			return 0
		})

		ctx.JSON(200, gin.H{
			"ok":             true,
			"bal_confirmed":  float64(confirmed) / Coin,
			"bal_pending":    float64(pending) / Coin,
			"bal_total":      float64(confirmed+pending) / Coin,
			"bal_owned":      float64(balance) / Coin,
			"debt":           (float64(confirmed+pending) - float64(balance)) / Coin,
			"debt_confirmed": (float64(confirmed) - float64(balance)) / Coin,
			"miners":         miners,
		})
	})

	err := r.Run("0.0.0.0:" + strconv.FormatInt(int64(cfg.Cfg.Master.ApiPort), 10))
	if err != nil {
		panic(err)
	}
}

type minerLog struct {
	Hashrate float64
	Wallet   string
}

func NotNan(n float64) float64 {
	if math.IsNaN(n) {
		return 0
	}
	return n
}

func Round0(n float64) float64 {
	return math.Round(n)
}
func Round3(n float64) float64 {
	return math.Round(n*1000) / 1000
}
func Round6(n float64) float64 {
	return math.Round(n*1000000) / 1000000
}
