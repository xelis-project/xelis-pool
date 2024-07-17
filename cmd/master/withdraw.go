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
	"math"
	"strings"
	"xelpool/cfg"
	"xelpool/config"
	"xelpool/database"
	"xelpool/log"
	"xelpool/util"

	"github.com/xelis-project/xelis-go-sdk/daemon"
	"github.com/xelis-project/xelis-go-sdk/wallet"
	bolt "go.etcd.io/bbolt"
)

func StartWallet() {
	/*httpClient, err := http.NewClient(http.ClientConfig{
		Username: config.WALLET_RPC_USERNAME,
		Password: config.WALLET_RPC_PASSWORD,
	})
	if err != nil {
		panic(err)
	}*/

	/*client, err := rpc.NewClient(cfg.Cfg.Master.WalletRpc) // rpc.WithHTTPClient(httpClient)
	if err != nil {
		panic(err)
	}

	WalletRpc = wallet.NewClient(client)*/

}

const DEV_TAX = 0

// If it returns false, there is an error (or nothing changed)
func CheckWithdraw() bool {
	log.Debug("CheckWithdraw()")
	defer log.Debug("CheckWithdraw() ended")

	balancesChanged := false

	err := DB.Update(func(tx *bolt.Tx) error {
		pendingBuck := tx.Bucket(database.PENDING)
		pendingBin := pendingBuck.Get([]byte("pending"))

		if pendingBin == nil {
			log.Debug("pendingBin is nil - there are no pending transactions")
			return nil
		}

		pending := database.PendingBals{}

		err := pending.Deserialize(pendingBin)
		if err != nil {
			log.Err(err)
			return err
		}

		if len(pending.UnconfirmedTxs) == 0 {
			log.Dev("len(pending.UnconfirmedTxs) is 0")
			return nil
		}

		MasterInfo.RLock()
		if pending.UnconfirmedTxs[0].UnlockHeight < MasterInfo.Height {
			MasterInfo.RUnlock()
			log.Info("pending block should have enough confirmations")

			log.Debugf("GetBlock with hash %x", pending.UnconfirmedTxs[0].TxnBlockHash[:])

			drpc := newDaemonRPC()
			txnBlock, err := drpc.GetBlockByHash(daemon.GetBlockByHashParams{
				Hash:       hex.EncodeToString(pending.UnconfirmedTxs[0].TxnBlockHash[:]),
				IncludeTxs: false,
			})
			if err != nil {
				log.Warn(err)

				// give it at most 10 blocks to get fixed, otherwise it's orphaned
				if pending.UnconfirmedTxs[0].UnlockHeight+10 < MasterInfo.Height {
					// block is probably orphaned
					log.Warn("block is very old, accounting it as orphaned")
					pending.UnconfirmedTxs = pending.UnconfirmedTxs[1:]
					log.Info("pending unconfirmedtxs", pending.UnconfirmedTxs)

					pendingBuck.Put([]byte("pending"), pending.Serialize())

					return nil
				} else {
					// wait more time
					return err
				}
			}

			blockType := strings.ToLower(txnBlock.BlockType)
			if blockType == "orphaned" {
				log.Warn("Block reward is orphaned - removing it, as this should not happen! Block hash is:", txnBlock.Hash)
				pending.UnconfirmedTxs = pending.UnconfirmedTxs[1:]
				pendingBuck.Put([]byte("pending"), pending.Serialize())
				return nil
			}

			log.Info("block type is:", blockType)

			// multiplier is used to avoid overpaying side blocks

			if txnBlock.MinerReward == nil {
				log.Err("txnBlock MinerReward is nil")

				return errors.New("txnBlock MinerReward is nil")
			}

			multiplier := (float64(*txnBlock.MinerReward) * (1 - DEV_TAX)) / float64(pending.UnconfirmedTxs[0].GetTotalMoney())

			log.Info("pending block multiplier:", multiplier)

			if multiplier > 1 {
				multiplier = 1
			}

			debt := GetDebt()
			log.Info("debt:", debt)
			if debt > 50 {
				log.Err("POOL HAS DEBT TO MINERS! Debt:", debt)
				log.Err("paying the block 2x to miners in order to compensate")
				multiplier *= 2
			} else if debt < -10 {
				log.Err("MINERS HAVE DEBT TO POOL! Debt:", debt)
				log.Err("paying the block 0.5x to miners in order to compensate")
				multiplier *= 0.5
			}

			infoBuck := tx.Bucket(database.ADDRESS_INFO)
			for i, v := range pending.UnconfirmedTxs[0].Bals {
				wallInfoBin := infoBuck.Get([]byte(i))

				addrInfo := database.AddrInfo{}

				if wallInfoBin == nil {
					log.Debug("wallInfoBin is nil")
				} else {
					err := addrInfo.Deserialize(wallInfoBin)
					if err != nil {
						log.Err(err)
						return err
					}
				}

				banned := false
				for _, v := range config.BANNED_ADDRESSES {
					if i == v {
						log.Warn("CheckWithdraw: banned address, setting its balance to 0")
						banned = true
						break
					}
				}

				addrInfo.Balance += uint64(float64(v) * multiplier)

				if banned {
					addrInfo.Balance = 0
					addrInfo.BalancePending = 0
				}

				err = infoBuck.Put([]byte(i), addrInfo.Serialize())
				if err != nil {
					return err
				}
			}

			if len(pending.UnconfirmedTxs) > 1 {
				pending.UnconfirmedTxs = pending.UnconfirmedTxs[1:]
			} else {
				pending.UnconfirmedTxs = []database.UnconfTx{}
			}

			balancesChanged = true
			return pendingBuck.Put([]byte("pending"), pending.Serialize())
		} else {
			MasterInfo.RUnlock()
			log.Dev("pending.UnconfirmedTxs[0] not confirmed yet - confirms in", pending.UnconfirmedTxs[0].UnlockHeight-MasterInfo.Height, "blocks")
			return nil
		}
	})
	if err != nil {
		log.Err(err)
	}

	return balancesChanged
}

const MIN_WITHDRAW_DESTINATIONS = 1
const MAX_WITHDRAW_DESTINATIONS = 25 // TODO

// returns true if there are still unpaid wallets
func Withdraw() (unpaid bool) {
	log.Info("Withdraw()")

	unpaid = false

	coin := math.Pow10(cfg.Cfg.Atomic)

	err := DB.Update(func(tx *bolt.Tx) error {
		buck := tx.Bucket(database.ADDRESS_INFO)

		var destinations []wallet.TransferOut
		var feeRevenue uint64

		curs := buck.Cursor()

		for key, val := curs.First(); key != nil; key, val = curs.Next() {
			if len(destinations) >= MAX_WITHDRAW_DESTINATIONS {
				log.Debug("Withdraw() unpaid:", unpaid)
				unpaid = true
				break
			}

			address := string(key)
			log.Dev("Withdraw: iterating over addresses. Current address is", address)

			addrInfo := database.AddrInfo{}

			err := addrInfo.Deserialize(val)
			if err != nil {
				log.Err(err)
				return err
			}

			log.Debug("Address has balance", float64(addrInfo.Balance)/coin)

			if addrInfo.Balance > uint64(cfg.Cfg.Master.MinWithdrawal*coin) {

				if address == cfg.Cfg.PoolAddress {
					log.Warn("Withdraw: address is PoolAddress, replacing it with fee address")
					address = cfg.Cfg.FeeAddress
				}
				for _, v := range config.BANNED_ADDRESSES {
					if address == v {
						log.Warn("Withdraw: banned address, replacing it with fee address")
						address = cfg.Cfg.FeeAddress
					}
				}

				fee := uint64(cfg.Cfg.Master.WithdrawalFee * coin)

				skip := false
				for _, v := range destinations {
					if v.Destination == address {
						log.Warn("Withdraw: address is already in destinations, skipping it")
						skip = true
					}
				}
				if skip {
					continue
				}

				// extra address validation step done by daemon
				/*valid, err := newDaemonRPC().ValidateAddress(daemon.ValidateAddressParams{
					Address:         address,
					AllowIntegrated: true,
				})
				if err != nil {
					log.Err("Withdraw: failed to check if address is valid:", err.Error()+", skipping it")
					continue
				}
				if !valid {
					log.Err("Withdraw: address", address, "is not valid")
					continue
				}*/
				log.Debug("checked address", address)

				// we can add the address to destinations
				destinations = append(destinations, wallet.TransferOut{
					Amount:      addrInfo.Balance - fee,
					Asset:       config.ASSET,
					Destination: address,
				})
				feeRevenue += fee

				addrInfo.Paid += addrInfo.Balance

				addrInfo.Balance = 0

				err = buck.Put(key, addrInfo.Serialize())
				if err != nil {
					log.Err(err)
					return err
				}
			}
		}

		if len(destinations) < MIN_WITHDRAW_DESTINATIONS {
			log.Warn("Not enough destinations for withdrawal")
			return nil
		}

		log.Info("Transferring to destinations", destinations)

		wrpc := newWalletRPC()

		data, err := wrpc.BuildTransaction(wallet.BuildTransactionParams{
			Transfers: destinations,
			Broadcast: true,
		})
		if err != nil {
			log.Err("transfer failed:", err)
			return err
		}
		log.Devf("Transfer result %x", data)

		Stats.Lock()
		Stats.RecentWithdrawals = append([]Withdrawal{
			{
				Txid:         data.Hash,
				Timestamp:    util.Time(),
				Destinations: destinations,
			},
		}, Stats.RecentWithdrawals...)
		Stats.Unlock()

		var txnFee uint64 = data.Fee

		log.Info("Payout txs total fee", float64(txnFee)/Coin)
		log.Info("Payout revenue fee  ", float64(feeRevenue)/Coin)
		log.Info("Earned ", float64(feeRevenue-txnFee)/Coin)

		if txnFee >= feeRevenue {
			log.Warn("Payout txs total fee is bigger than the revenue fee. Consider increasing withdrawal_fee.")
			feeRevenue = 0
		} else {
			feeRevenue -= txnFee
		}

		feeAddrData := buck.Get([]byte(cfg.Cfg.FeeAddress))
		feeAddr := database.AddrInfo{}

		if feeAddrData != nil {
			err := feeAddr.Deserialize(feeAddrData)
			if err != nil {
				log.Err(err)
				return err
			}
		}

		feeAddr.Balance += feeRevenue

		return nil
	})
	if err != nil {
		log.Err(err)
		unpaid = false
	}

	return unpaid
}
