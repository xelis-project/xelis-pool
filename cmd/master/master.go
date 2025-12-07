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
	"context"
	"math"
	"net"
	"strconv"
	"sync"
	"xelis-pool/address"
	"xelis-pool/cfg"
	"xelis-pool/config"
	"xelis-pool/database"
	"xelis-pool/log"
	"xelis-pool/util"

	"github.com/xelis-project/xelis-go-sdk/daemon"
	"github.com/xelis-project/xelis-go-sdk/wallet"
	bolt "go.etcd.io/bbolt"
)

type Info struct {
	BlockReward uint64
	Height      uint64

	sync.RWMutex
}

var MasterInfo Info

var DB *bolt.DB

func newDaemonRPC() *daemon.RPC {
	rp, err := daemon.NewRPC(context.Background(), "http://"+cfg.Cfg.Master.DaemonRpc+"/json_rpc")
	if err != nil {
		log.Fatal(err)
	}
	return rp
}
func newWalletRPC() *wallet.RPC {
	rp, err := wallet.NewRPC(context.Background(), "http://"+cfg.Cfg.Master.WalletRpc+"/json_rpc",
		cfg.Cfg.Master.WalletRpcUser, cfg.Cfg.Master.WalletRpcPass)
	if err != nil {
		log.Fatal(err)
	}
	return rp
}

func main() {
	if !address.IsAddressValid(cfg.Cfg.PoolAddress) {
		log.Fatal("Pool address is not valid")
	}
	if !address.IsAddressValid(cfg.Cfg.FeeAddress) {
		log.Fatal("Fee address is not valid")
	}
	var err error
	DB, err = bolt.Open("pool.db", 0o600, bolt.DefaultOptions)

	if err != nil {
		log.Fatal(err)
	}

	go startDiscord()

	err = DB.Update(func(tx *bolt.Tx) error {
		_, err := tx.CreateBucketIfNotExists(database.ADDRESS_INFO)
		if err != nil {
			return err
		}
		_, err = tx.CreateBucketIfNotExists(database.PENDING)
		if err != nil {
			return err
		}
		_, err = tx.CreateBucketIfNotExists(database.SHARES)

		return err
	})
	if err != nil {
		log.Fatal(err)
	}

	DatabaseCleanup()

	StartWallet()

	srv, err := net.Listen("tcp", config.MASTER_SERVER_HOST+":"+strconv.FormatUint(uint64(cfg.Cfg.Master.Port), 10))
	if err != nil {
		panic(err)
	}
	log.Info("Master server listening on", config.MASTER_SERVER_HOST)

	log.Info("Using daemon RPC " + cfg.Cfg.Master.DaemonRpc)

	go StartApiServer()
	go StatsServer()

	go Updater()

	for {
		conn, err := srv.Accept()
		if err != nil {
			log.Err(err)
		}
		go HandleSlave(conn)
	}
}

func DatabaseCleanup() {
	log.Info("Starting database cleanup")

	var sharesRemoved = 0
	var sharesKept = 0

	err := DB.Update(func(tx *bolt.Tx) error {
		buck := tx.Bucket(database.SHARES)

		cursor := buck.Cursor()

		for k, v := cursor.First(); k != nil; k, v = cursor.Next() {
			sh := database.Share{}

			err := sh.Deserialize(v)

			if err != nil {
				log.Warn("error reading share:", err)
				buck.Delete(k)
				sharesRemoved++
				continue
			}

			// delete outdated shares
			if sh.Time+GetPplnsWindow() < util.Time() {
				buck.Delete(k)
				sharesRemoved++
				continue
			}

			sharesKept++
		}

		return nil
	})
	if err != nil {
		log.Err(err)
	}

	log.Info("Database cleanup OK,", sharesRemoved, "outdated shares removed,", sharesKept, "maintained")
}

func OnShareFound(ip string, wallet string, diff uint64, numShares uint32) {
	if !address.IsAddressValid(wallet) {
		log.Warn("Wallet", wallet, "is not valid. Replacing it with fee address.")
		wallet = cfg.Cfg.FeeAddress
	}
	for _, v := range config.BANNED_ADDRESSES {
		if wallet == v {
			log.Warn("slave "+ip+": wallet", wallet, "is banned, ignoring the share")
			return
		}
	}

	Stats.Lock()
	kwall := Stats.KnownAddresses[wallet]

	kwall.AddShare(float64(diff), util.TimePrecise())

	log.Info("slave "+ip+": Wallet", wallet, "found", numShares, "shares with diff", float64(diff/100)/10, "k HR:", Stats.GetHashrate(wallet))

	Stats.KnownAddresses[wallet] = kwall

	Stats.Hashes += float64(diff)
	Stats.Cleanup()
	Stats.Unlock()

	DB.Update(func(tx *bolt.Tx) error {
		buck := tx.Bucket(database.SHARES)

		shareId, _ := buck.NextSequence()

		shareData := database.Share{
			Wallet: wallet,
			Diff:   diff,
			Time:   util.Time(),
		}

		buck.Put(util.Itob(shareId), shareData.Serialize())

		return nil
	})
}

// Important: Stats must be locked and MasterInfo must not be locked
func GetEstPendingBalance(addr string) float64 {

	var totHashes float64
	var minerHashes float64

	MasterInfo.RLock()
	reward := float64(MasterInfo.BlockReward) / math.Pow10(cfg.Cfg.Atomic)
	MasterInfo.RUnlock()

	balance := minerHashes / totHashes * reward

	return balance
}
