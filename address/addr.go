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

package address

import (
	"xelpool/cfg"
	"xelpool/log"
	"xelpool/xelisutil/bech32"
)

const PREFIX = "xel:"

func IsAddressValid(addr string) bool {
	prefix, data, err := bech32.Decode(addr)
	if err != nil {
		log.Debugf("address is not valid: %s", err)
		return false
	}

	if prefix != cfg.Cfg.AddressPrefix {
		return false
	}
	if len(data) < 53 || len(data) > 70 {
		log.Warnf("address data %x is less than 53 or more than 70 bytes long", data)
		return false
	}

	return true
}
