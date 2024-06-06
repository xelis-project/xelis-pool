// Package serializer implements a simple and useful tool
// for serialization/deserialization of binary data
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

package serializer

import (
	"encoding/binary"
	"math/big"
	"runtime"
	"strconv"
	"strings"
)

type Serializer struct {
	Data []byte
}

func (s *Serializer) AddUint8(n uint8) {
	s.Data = append(s.Data, n)
}
func (s *Serializer) AddUint16(n uint16) {
	s.Data = binary.LittleEndian.AppendUint16(s.Data, n)
}
func (s *Serializer) AddUint32(n uint32) {
	s.Data = binary.LittleEndian.AppendUint32(s.Data, n)
}
func (s *Serializer) AddUint64(n uint64) {
	s.Data = binary.LittleEndian.AppendUint64(s.Data, n)
}

func (s *Serializer) AddUvarint(n uint64) {
	s.Data = binary.AppendUvarint(s.Data, n)
}

func (s *Serializer) AddFixedByteArray(a []byte, length int) {
	if len(a) != length {
		panic("invalid length")
	}
	s.Data = append(s.Data, a...)
}
func (s *Serializer) AddByteSlice(a []byte) {
	s.Data = append(binary.AppendUvarint(s.Data, uint64(len(a))), a...)
}
func (s *Serializer) AddString(a string) {
	s.AddByteSlice([]byte(a))
}
func (s *Serializer) AddBigInt(n *big.Int) {
	bin := n.Bytes()

	s.Data = append(binary.AppendUvarint(s.Data, uint64(len(bin))), bin...)
}
func (s *Serializer) AddBool(b bool) {
	if b {
		s.Data = append(s.Data, 1)
	} else {
		s.Data = append(s.Data, 0)
	}
}

func GetCaller() string {
	_, file, line, _ := runtime.Caller(2)
	fileSpl := strings.Split(file, "/")
	return fileSpl[len(fileSpl)-1] + ":" + strconv.FormatInt(int64(line), 10)

}
