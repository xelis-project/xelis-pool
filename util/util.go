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

package util

import (
	"crypto/rand"
	"encoding/binary"
	"encoding/json"
	"strconv"
	"strings"
	"time"
)

func RemovePort(s string) string {
	return strings.Split(s, ":")[0]
}

func RandomUint64() uint64 {
	b := make([]byte, 8)
	rand.Read(b)

	return binary.BigEndian.Uint64(b)
}

func Uint64ToBigEndian(n uint64) []byte {
	b := make([]byte, 8)
	binary.BigEndian.PutUint64(b, n)
	return b
}

func Itob(n uint64) []byte {
	b := make([]byte, 8)
	binary.LittleEndian.PutUint64(b, n)
	return b
}

// returns a random float between 0 and 1
func RandomFloat() float32 {
	b := make([]byte, 4)
	rand.Read(b)

	return float32(binary.LittleEndian.Uint32(b)) / 0xffffffff
}

func Time() uint64 {
	return uint64(time.Now().Unix())
}

func TimePrecise() float64 {
	return float64(time.Now().UnixNano()) / float64(time.Second.Nanoseconds())
}

func DumpJson(d any) string {

	data, err := json.MarshalIndent(d, "", "\t")
	if err != nil {
		panic(err)
	}

	return string(data)
}

func FormatUint[K uint | uint8 | uint16 | uint32 | uint64](n K) string {
	return strconv.FormatUint(uint64(n), 10)
}
func FormatInt[K int | int8 | int16 | int32 | int64](n K) string {
	return strconv.FormatInt(int64(n), 10)
}
