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

package database

import "xelpool/serializer"

type Share struct {
	Wallet string `json:"wall"`
	Diff   uint64 `json:"diff"`
	Time   uint64 `json:"time"`
}

const VERSION = 0

func (x *Share) Serialize() []byte {
	s := serializer.Serializer{}

	s.AddUint8(VERSION)

	s.AddString(x.Wallet)
	s.AddUint64(x.Diff)
	s.AddUint64(x.Time)

	return s.Data
}
func (x *Share) Deserialize(data []byte) error {
	d := serializer.Deserializer{
		Data: data,
	}

	d.ReadUint8()

	x.Wallet = d.ReadString()
	x.Diff = d.ReadUint64()
	x.Time = d.ReadUint64()

	return d.Error
}

type UnconfTx struct {
	UnlockHeight uint64
	TxnBlockHash [32]byte
	Bals         map[string]uint64
}
type PendingBals struct {
	LastHeight uint64

	UnconfirmedTxs []UnconfTx
}

func (x *UnconfTx) Serialize() []byte {
	s := serializer.Serializer{}

	s.AddUint8(VERSION)
	s.AddUvarint(x.UnlockHeight)
	s.AddFixedByteArray(x.TxnBlockHash[:], 32)
	s.AddUvarint(uint64(len(x.Bals)))
	for i, v := range x.Bals {
		s.AddString(i)
		s.AddUvarint(v)
	}

	return s.Data
}
func (x *UnconfTx) Deserialize(data []byte) ([]byte, error) {
	d := serializer.Deserializer{
		Data: data,
	}

	d.ReadUint8()

	x.UnlockHeight = d.ReadUvarint()
	x.TxnBlockHash = [32]byte(d.ReadFixedByteArray(32))

	balsLen := int(d.ReadUvarint())

	x.Bals = make(map[string]uint64, balsLen)

	for i := 0; i < balsLen; i++ {
		x.Bals[d.ReadString()] = d.ReadUvarint()
	}

	return d.Data, d.Error
}
func (x *UnconfTx) GetTotalMoney() uint64 {
	var money uint64 = 0
	for _, v := range x.Bals {
		money += v
	}
	return money
}

func (x *PendingBals) Serialize() []byte {
	s := serializer.Serializer{}

	s.AddUint8(VERSION)

	s.AddUvarint(x.LastHeight)

	s.AddUvarint(uint64(len(x.UnconfirmedTxs)))

	for _, v := range x.UnconfirmedTxs {
		s.Data = append(s.Data, v.Serialize()...)
	}

	return s.Data
}

func (x *PendingBals) Deserialize(data []byte) error {
	d := serializer.Deserializer{
		Data: data,
	}

	d.ReadUint8()

	x.LastHeight = d.ReadUvarint()

	numUnconf := int(d.ReadUvarint())

	x.UnconfirmedTxs = make([]UnconfTx, 0, numUnconf)

	for i := 0; i < numUnconf; i++ {
		utx := UnconfTx{}
		var err error
		d.Data, err = utx.Deserialize(d.Data)
		if err != nil {
			return err
		}
		x.UnconfirmedTxs = append(x.UnconfirmedTxs, utx)
	}

	return d.Error
}

// AddrInfo holds informations about a given address
type AddrInfo struct {
	Balance        uint64 // confirmed balance that can be paid out
	BalancePending uint64 // unconfirmed balance (cannot be paid out)
	Paid           uint64 // amount that has been paid out so far
}

func (x *AddrInfo) Serialize() []byte {
	s := serializer.Serializer{}

	s.AddUint8(VERSION)

	s.AddUvarint(x.Balance)
	s.AddUvarint(x.BalancePending)
	s.AddUvarint(x.Paid)

	return s.Data
}

func (x *AddrInfo) Deserialize(data []byte) error {
	d := serializer.Deserializer{
		Data: data,
	}

	d.ReadUint8()

	x.Balance = d.ReadUvarint()
	x.BalancePending = d.ReadUvarint()
	x.Paid = d.ReadUvarint()

	return d.Error
}

/*
database structure:

addressInfo: address -> address data
shares: share id -> share data
*/

var (
	ADDRESS_INFO = []byte("a") // address -> address data
	SHARES       = []byte("s") // share id -> share data
	PENDING      = []byte("p") // "pending" -> pending balances
)
