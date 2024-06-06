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

package xatum

import (
	"encoding/base64"
	"errors"
)

type B64 []byte

func (m B64) Marshal() ([]byte, error) {
	return []byte(`"` + base64.StdEncoding.EncodeToString(m) + `"`), nil
}
func (m *B64) UnmarshalJSON(c []byte) error {
	if c == nil || len(c) < 2 {
		return errors.New("value is too short")
	} else if len(c) == 2 {
		*m = append((*m)[0:0], []byte{}...)
		return nil
	}

	if c[0] != '"' || c[len(c)-1] != '"' {
		return errors.New("invalid string literal")
	}

	dst := make([]byte, base64.StdEncoding.EncodedLen(len(c)))

	n, err := base64.StdEncoding.Decode(dst, c[1:len(c)-1])

	*m = append((*m)[0:0], dst[:n]...)

	return err
}
