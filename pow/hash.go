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

package pow

import (
	"errors"
	"strings"

	xelis_hash "github.com/xelis-project/xelis-hash/go"
	"github.com/zeebo/blake3"
)

var ErrUnsupportedPOWAlgorithm = errors.New("unsupported POW algorithm")

func FastHash(d []byte) [32]byte {
	return blake3.Sum256(d)
}

func ConvertAlgorithmToStratum(algorithm string) (string, error) {
	algorithm = strings.ToLower(algorithm)
	switch algorithm {
	case "xel/v1":
		return "xel/0", nil
	case "xel/v2":
		return "xel/1", nil
	case "xel/v3":
		return "xel/2", nil
	default:
		// check if its a stratum algo already
		switch algorithm {
		case "xel/0", "xel/1", "xel/2":
			return algorithm, nil
		default:
			return "", ErrUnsupportedPOWAlgorithm
		}
	}
}

func ConvertAlgorithmToGetwork(algorithm string) (string, error) {
	algorithm = strings.ToLower(algorithm)
	switch algorithm {
	case "xel/0":
		return "xel/v1", nil
	case "xel/1":
		return "xel/v2", nil
	case "xel/2":
		return "xel/v3", nil
	default:
		// check if its a getwork algo already
		switch algorithm {
		case "xel/v1", "xel/v2", "xel/v3":
			return algorithm, nil
		default:
			return "", ErrUnsupportedPOWAlgorithm
		}
	}
}

func PowHash(d []byte, algorithm string) ([32]byte, error) {
	// ignore the error
	algorithm, _ = ConvertAlgorithmToGetwork(algorithm)

	switch algorithm {
	case "xel/v1":
		return xelis_hash.HashV1(d)
	case "xel/v2":
		return xelis_hash.HashV2(d)
	case "xel/v3":
		return xelis_hash.HashV3(d)
	default:
		return [32]byte{}, ErrUnsupportedPOWAlgorithm
	}
}
