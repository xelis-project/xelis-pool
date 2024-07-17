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

	"github.com/xelis-project/xelis-go-sdk/address"
)

func IsAddressValid(addr string) bool {
	address.PrefixAddress = cfg.Cfg.AddressPrefix
	valid, err := address.IsValidAddress(addr)
	if err != nil || !valid {
		log.Debugf("address is not valid: %s", err)
		return false
	}

	return true
}
