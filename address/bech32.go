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
	"strings"
)

var CHARSET = "qpzry9x8gf2tvdw0s3jn54khce6mua7l"
var GENERATOR = [5]uint32{0x3b6a57b2, 0x26508e6d, 0x1ea119fa, 0x3d4233dd, 0x2a1462b3}
var SEPARATOR = ':'

type Bech32Error struct {
	Message string
}

func (e *Bech32Error) Error() string {
	return e.Message
}

func NewBech32Error(msg string) error {
	return &Bech32Error{Message: msg}
}

func polymod(values []byte) uint32 {
	chk := uint32(1)
	for _, value := range values {
		top := chk >> 25
		chk = (chk&0x1ffffff)<<5 ^ uint32(value)
		for i, item := range GENERATOR {
			if (top>>uint(i))&1 == 1 {
				chk ^= item
			}
		}
	}
	return chk
}

func hrpExpand(hrp string) []byte {
	var result []byte
	for _, c := range hrp {
		result = append(result, byte(c>>5))
	}
	result = append(result, 0)
	for _, c := range hrp {
		result = append(result, byte(c&31))
	}
	return result
}

func verifyChecksum(hrp string, data []byte) bool {
	vec := hrpExpand(hrp)
	vec = append(vec, data...)
	return polymod(vec) == 1
}

func createChecksum(hrp string, data []byte) [6]byte {
	values := hrpExpand(hrp)
	values = append(values, data...)
	var result [6]byte
	values = append(values, result[:]...)
	polymodValue := polymod(values) ^ 1
	for i := 0; i < 6; i++ {
		result[i] = byte((polymodValue >> uint(5*(5-i)) & 31))
	}
	return result
}

func Encode(hrp string, data []byte) (string, error) {
	if len(hrp) == 0 {
		return "", NewBech32Error("Human readable part is empty")
	}
	for _, value := range hrp {
		if value < 33 || value > 126 {
			return "", NewBech32Error("Invalid character value in human readable part")
		}
	}
	if strings.ToUpper(hrp) != hrp && strings.ToLower(hrp) != hrp {
		return "", NewBech32Error("Mix case is not allowed in human readable part")
	}
	hrp = strings.ToLower(hrp)
	createdChecksum := createChecksum(hrp, data)
	combined := append(data, createdChecksum[:]...)
	var result []byte
	result = append(result, []byte(hrp)...)
	result = append(result, byte(SEPARATOR))
	for _, value := range combined {
		if value > byte(len(CHARSET)) {
			return "", NewBech32Error("Invalid value")
		}
		result = append(result, CHARSET[value])
	}
	return string(result), nil
}

func Decode(bech string) (string, []byte, error) {
	if strings.ToUpper(bech) != bech && strings.ToLower(bech) != bech {
		return "", nil, NewBech32Error("Mix case is not allowed in human readable part")
	}
	pos := strings.LastIndex(bech, string(SEPARATOR))
	if pos < 1 || pos+7 > len(bech) {
		return "", nil, NewBech32Error("Invalid separator position")
	}
	hrp := bech[0:pos]
	for _, value := range hrp {
		if value < 33 || value > 126 {
			return "", nil, NewBech32Error("Invalid character value in human readable part")
		}
	}
	var data []byte
	for i := pos + 1; i < len(bech); i++ {
		c := rune(bech[i])
		value := strings.IndexRune(CHARSET, c)
		if value == -1 {
			return "", nil, NewBech32Error("Invalid value")
		}
		data = append(data, byte(value))
	}
	if !verifyChecksum(hrp, data) {
		return "", nil, NewBech32Error("Invalid checksum")
	}

	data = data[:len(data)-6]

	return hrp, data, nil
}
