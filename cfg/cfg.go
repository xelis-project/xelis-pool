// Copyright (C) 2024 duggavo
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

package cfg

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"xelpool/log"
)

var Cfg Config

func init() {
	fd, err := os.ReadFile("config.json")
	if err != nil {
		fmt.Println(err)

		fd, err = os.ReadFile("../config.json")
		if err != nil {
			blankCfg, err := json.MarshalIndent(Config{}, "", "\t")

			if err != nil {
				panic(err)
			}

			os.WriteFile("config.json", blankCfg, 0o666)

			panic(fmt.Errorf("could not open config: %s. blank configuration created", err))
		}
	}

	err = json.Unmarshal(fd, &Cfg)
	if err != nil {
		panic(err)
	}

	log.LogLevel = Cfg.LogLevel

	// master password is hashed with sha256 to make it fixed-length (32 bytes long)
	MasterPass = sha256.Sum256([]byte(Cfg.MasterPass))
}

var MasterPass [32]byte

type Config struct {
	LogLevel   uint8
	MasterPass string
	Atomic     int

	PoolAddress string
	FeeAddress  string

	BlockTime uint64

	AddressPrefix string

	Slave  Slave
	Master Master
}

type Slave struct {
	MasterAddress string

	InitialDifficulty uint64
	MinDifficulty     uint64
	ShareTarget       float64

	XatumPort   uint16
	GetworkPort uint16
	StratumPort uint16

	TrustScore         int32
	TrustedCheckChance float32
}

type Master struct {
	Port       uint16
	ApiPort    uint16
	FeePercent float64

	MinWithdrawal float64
	WithdrawalFee float64

	MinConfs uint64

	WalletRpc string
	DaemonRpc string

	WalletRpcUser string
	WalletRpcPass string

	DiscordWebhook string
}
