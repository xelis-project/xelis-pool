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
	return PowHashV1(d)
}
func PowHashV1(d []byte) [32]byte {
	if len(d) > xelishash.BYTES_ARRAY_INPUT {
		panic("PowHash input is too long")
	}
	buf := make([]byte, xelishash.BYTES_ARRAY_INPUT)
	copy(buf, d)
	data := threadpool.XelisHash(buf)
	return data
}
func PowHashV2(d []byte) [32]byte {
	data := threadpool.XelisHashV2(d)
	return data
}
