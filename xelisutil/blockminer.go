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

package xelisutil

import (
	"encoding/binary"
	"errors"
	"fmt"
	"time"

	"github.com/duggavo/serializer"
	"github.com/xelpool/xelishash"
)

// Xatum Protocol BlockMiner implementation

const BLOCKMINER_LENGTH = 112

type BlockMiner [BLOCKMINER_LENGTH]byte

func NewBlockMiner(workhash, extranonce, publickey [32]byte) BlockMiner {
	s := serializer.Serializer{
		Endian: binary.BigEndian,
	}

	s.AddFixedByteArray(workhash[:], 32)
	s.AddUint64(uint64(time.Now().UnixMilli()))
	s.AddUint64(0)
	s.AddFixedByteArray(extranonce[:], 32)
	s.AddFixedByteArray(publickey[:], 32)

	return BlockMiner(s.Data)

}
func NewBlockMinerFromBlob(blob []byte) (BlockMiner, error) {
	if len(blob) != 96 {
		return BlockMiner{}, errors.New("malformed BlockMinerBlob")
	}

	return NewBlockMiner([32]byte(blob[0:32]), [32]byte(blob[32:32*2]), [32]byte(blob[32*2:32*3])), nil

}

// SETTER methods
func (b *BlockMiner) SetTimestamp(t uint64) {
	tb := make([]byte, 8)
	binary.BigEndian.PutUint64(tb, t)

	// update the timestamp
	b[32] = tb[0]
	b[33] = tb[1]
	b[34] = tb[2]
	b[35] = tb[3]
	b[36] = tb[4]
	b[37] = tb[5]
	b[38] = tb[6]
	b[39] = tb[7]
}
func (b *BlockMiner) SetNonce(n uint64) {
	tb := make([]byte, 8)
	binary.BigEndian.PutUint64(tb, n)

	// update the nonce
	b[40] = tb[0]
	b[41] = tb[1]
	b[42] = tb[2]
	b[43] = tb[3]
	b[44] = tb[4]
	b[45] = tb[5]
	b[46] = tb[6]
	b[47] = tb[7]
}

func (b *BlockMiner) SetExtraNonce(n [32]byte) {
	for i := 0; i < 32; i++ {
		b[48+i] = n[i]
	}
}

// GETTER methods

func (b BlockMiner) Serialize() []byte {
	return b[:]
}
func (b BlockMiner) Hash() [32]byte {
	return FastHash(b[:])
}
func (b BlockMiner) PowHash(sp *xelishash.ScratchPad) [32]byte {
	return PowHash(b[:], sp)
}

func (b BlockMiner) GetWorkhash() [32]byte {
	return [32]byte(b[:32])
}
func (b BlockMiner) GetTimestamp() uint64 {
	return binary.BigEndian.Uint64(b[32:40])
}
func (b BlockMiner) GetNonce() uint64 {
	return binary.BigEndian.Uint64(b[40:48])
}
func (b BlockMiner) GetExtraNonce() [32]byte {
	return [32]byte(b[48:80])
}
func (b BlockMiner) GetPublickey() [32]byte {
	return [32]byte(b[80:112])
}

func (b BlockMiner) GetBlob() []byte {
	wh := b.GetWorkhash()
	xn := b.GetExtraNonce()
	pk := b.GetPublickey()
	return append(append(wh[:], xn[:]...), pk[:]...)
}

// returns job id, which is the first 16 bytes of ExtraNonce, only used by Stratum
func (b BlockMiner) GetJobID() [16]byte {
	return [16]byte(b[48 : 48+16])
}

// sets job id, which is the first 16 bytes of ExtraNonce, only used by Stratum
func (b *BlockMiner) SetJobID(n [16]byte) {
	for i := 0; i < 16; i++ {
		b[48+i] = n[i]
	}
}

// returns pool nonce, which is the 8 bytes after JobID
func (b BlockMiner) GetPoolNonce() [8]byte {
	return [8]byte(b[48+16 : 48+16+8])
}

// sets pool nonce, which is the 8 after JobID
func (b *BlockMiner) SetPoolNonce(n [8]byte) {
	for i := 0; i < 8; i++ {
		b[48+16+i] = n[i]
	}
}

func (b *BlockMiner) ToString() string {
	return fmt.Sprintf("timestamp: %x\nnonce: %x\nnonce extra: %x\n"+
		"public key: %x\nwork hash: %x", b.GetTimestamp(), b.GetNonce(),
		b.GetExtraNonce(), b.GetPublickey(), b.GetWorkhash())
}
