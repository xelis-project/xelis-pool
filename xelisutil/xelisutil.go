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

package xelisutil

import (
	"runtime"
	"xelpool/log"

	"github.com/xelpool/xelishash"
	"github.com/zeebo/blake3"
)

var threadpool *xelishash.ThreadPool

func init() {
	threadpool = xelishash.NewThreadPool(runtime.NumCPU())
}

func FastHash(d []byte) [32]byte {
	return blake3.Sum256(d)
}

func PowHash(d []byte, algo string) [32]byte {
	if algo == "xel/1" {
		return PowHashV2(d)
	}
	log.Err("V1 hash is disabled in source code")
	return [32]byte(maxBigInt.Bytes())
}
func PowHashV2(d []byte) [32]byte {
	data := threadpool.XelisHashV2(d)
	return data
}
