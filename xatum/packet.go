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
	"encoding/json"
	"errors"
	"strings"
)

type Packet struct {
	Name string
	Data any
}

func NewPacket(name string, data any) Packet {
	return Packet{
		Name: name,
		Data: data,
	}
}
func NewPacketFromString(data string, pack *Packet) error {
	spl := strings.SplitN(data, "~", 2)
	if spl == nil || len(spl) < 2 {
		return errors.New("malformed packet string")
	}

	pack.Name = spl[0]

	err := json.Unmarshal([]byte(spl[1]), &pack.Data)

	return err
}

func (p Packet) ToString() (string, error) {
	data, err := json.Marshal(p.Data)
	if err != nil {
		return "", err
	}

	return p.Name + "~" + string(data), nil
}
