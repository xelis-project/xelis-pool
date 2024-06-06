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

package slave

import (
	"time"
	"xelpool/log"
	"xelpool/mut"
	"xelpool/serializer"
)

type ShareCache struct {
	NumShares uint32
	TotalDiff uint64
}

type Cache struct {
	Shares map[string]ShareCache

	mut.RWMutex
}

var slaveCache = Cache{
	Shares: map[string]ShareCache{},
}

func cacheShare(wallet string, diff uint64) {
	slaveCache.Lock()
	defer slaveCache.Unlock()

	x := slaveCache.Shares[wallet]

	x.NumShares++
	x.TotalDiff += diff

	slaveCache.Shares[wallet] = x
}

func init() {
	go func() {
		for {
			time.Sleep(5 * time.Second)

			connMut.Lock()
			if conn != nil {
				slaveCache.Lock()
				length := len(slaveCache.Shares)
				for i, v := range slaveCache.Shares {
					log.Debug("sending cache share with address:", i, "count", v.NumShares, "total diff", v.TotalDiff)
					sendCachedShare(v.NumShares, i, v.TotalDiff)
				}
				slaveCache.Shares = make(map[string]ShareCache, length+10)
				slaveCache.Unlock()
			}
			connMut.Unlock()
		}
	}()
}

func sendCachedShare(count uint32, wallet string, diff uint64) {
	s := serializer.Serializer{
		Data: []byte{0},
	}

	s.AddUvarint(uint64(count))
	s.AddString(wallet)
	s.AddUvarint(diff)

	sendToConn(s.Data)
}
