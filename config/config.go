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

package config

import (
	"crypto/rand"
	"xelpool/log"
)

var Verbose bool

const ALGO = "xel/0"

const MAX_REQUEST_SIZE = 5 * 1024 // 5 MiB
const TIMEOUT = 5
const SLAVE_MINER_TIMEOUT = 30

// in seconds
const TIMESTAMP_FUTURE_LIMIT = 10

const MASTER_SERVER_HOST = "0.0.0.0"

// seconds
const WITHDRAW_INTERVAL = 8 * 60 * 60 // withdraw once every 8 hours

const ASSET = "0000000000000000000000000000000000000000000000000000000000000000"

const MAX_PAST_JOBS = 6

const MAX_CONNECTIONS_PER_IP = 100

var BANNED_ADDRESSES = []string{
	"xel:z6fe7y88pfmep7lngvrmqdqma980qyr6xr56ylnu0w4pyfmaqpcqqhjf3zv",
	"xel:ecfplm3wq8xsqjpa03vjpyhfx35hytjskglat2y6jy7jmleta4zsqurvtcx", // invalid share hacker, 223.73.162.31
}

const MAX_DIFFICULTY = 10_000_000_000 // 10G max difficulty

// AUTOMATICALLY SET - data stored in the second part of extra nonce for Stratum jobs
var POOL_NONCE = [8]byte{}

func init() {
	_, err := rand.Read(POOL_NONCE[:])
	if err != nil {
		log.Fatal(err)
	}
	log.Infof("POOL_NONCE: %x", POOL_NONCE)
}
