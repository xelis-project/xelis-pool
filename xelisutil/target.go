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
	"bytes"
	"math/big"
	"xelpool/util"
)

var maxBigInt *big.Int

func init() {

	b, _ := big.NewInt(0).SetString("ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff", 16)

	maxBigInt = b

}

func GetTarget(diff uint64) *big.Int {
	if diff == 0 {
		return big.NewInt(0)
	}

	diffBigInt := big.NewInt(0)
	diffBigInt = diffBigInt.SetBytes(util.Uint64ToBigEndian(diff))

	return diffBigInt.Div(maxBigInt, diffBigInt)
}

func GetTargetBytes(diff uint64) [32]byte {
	data := make([]byte, 32)

	byt := GetTarget(diff).Bytes()

	copy(data[32-len(byt):], byt)

	return [32]byte(data)
}

// returns true if the hash matches difficulty
func CheckDiff(hash [32]byte, diff uint64) bool {
	target := GetTargetBytes(diff)

	return bytes.Compare(hash[:], target[:]) < 0
}
