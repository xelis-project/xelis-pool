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

package rate_limit

import (
	"sync"
	"xelpool/config"
)

var connsPerIp = make(map[string]uint32, 100)
var connsMut sync.RWMutex

// returns true and increases IP connections by 1 if the miner can connect,
// otherwise returns false and does not increase number of connections
func CanConnect(ip string) bool {
	connsMut.Lock()
	defer connsMut.Unlock()

	if connsPerIp[ip] > config.MAX_CONNECTIONS_PER_IP {
		return false
	}
	connsPerIp[ip]++

	return true
}
func Disconnect(ip string) {
	connsMut.Lock()
	defer connsMut.Unlock()

	if connsPerIp[ip] > 0 {
		connsPerIp[ip]--
	}
}
