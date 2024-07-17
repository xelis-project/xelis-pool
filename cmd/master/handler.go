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

package main

import (
	"encoding/binary"
	"encoding/hex"
	"io"
	"net"
	"time"
	"xelpool/log"
	"xelpool/serializer"
	"xelpool/util"
)

const Overhead = 40

// numConns is locked by the mutex of Stats
var numConns = make(map[uint64]uint32)

func HandleSlave(conn net.Conn) {
	var connId uint64 = util.RandomUint64()

	for {
		lenBuf := make([]byte, 2+Overhead)
		_, err := io.ReadFull(conn, lenBuf)
		if err != nil {
			log.Warn(err)
			conn.Close()
			Stats.Lock()
			delete(numConns, connId)
			Stats.Unlock()
			return
		}
		lenBuf, err = Decrypt(lenBuf)
		if err != nil {
			log.Warn(err)
			conn.Close()
			Stats.Lock()
			delete(numConns, connId)
			Stats.Unlock()
			return
		}
		len := int(lenBuf[0]) | (int(lenBuf[1]) << 8)

		// read the actual message

		buf := make([]byte, len+Overhead)
		_, err = io.ReadFull(conn, buf)
		if err != nil {
			log.Warn(err)
			conn.Close()
			Stats.Lock()
			delete(numConns, connId)
			Stats.Unlock()
			return
		}
		buf, err = Decrypt(buf)
		if err != nil {
			log.Warn(err)
			conn.Close()
			Stats.Lock()
			delete(numConns, connId)
			Stats.Unlock()
			return
		}
		log.NetDev("Received message:", hex.EncodeToString(buf))
		OnMessage(buf, connId, conn)
	}
}

func SendToConn(conn net.Conn, data []byte) {
	var dataLenBin = make([]byte, 0, 2)
	dataLenBin = binary.LittleEndian.AppendUint16(dataLenBin, uint16(len(data)))
	conn.Write(Encrypt(dataLenBin))
	conn.Write(Encrypt(data))
}

// Stats MUST NOT be locked before calling this
func OnMessage(msg []byte, connId uint64, conn net.Conn) {
	d := serializer.Deserializer{
		Data: msg,
	}
	if d.Error != nil {
		log.Err(d.Error)
		return
	}

	packet := d.ReadUint8()

	switch packet {
	case 0: // Share Found packet
		numShares := uint32(d.ReadUvarint())
		wallet := d.ReadString()
		diff := d.ReadUvarint()

		if d.Error != nil {
			log.Err(d.Error)
			return
		}

		OnShareFound(conn.RemoteAddr().String(), wallet, diff, numShares)
	case 1: // Block Found packet
		hash := hex.EncodeToString(d.ReadFixedByteArray(32))

		if d.Error != nil {
			log.Err(d.Error)
			return
		}

		log.Info("Found block with hash", hash)
		go func() {
			time.Sleep(10 * time.Second) // add delay to allow daemon to process the block
			OnBlockFound(hash)
		}()
	case 2: // Stats packet
		conns := uint32(d.ReadUvarint())

		if d.Error != nil {
			log.Err(d.Error)
			return
		}

		Stats.Lock()
		numConns[connId] = conns
		Stats.Workers = 0
		for _, v := range numConns {
			Stats.Workers += v
		}
		Stats.Unlock()
	case 4: // Ban
		bannedIp := d.ReadString()
		banEnds := d.ReadUint64()

		Stats.Lock()

		SendToConn(conn, banM2S{
			Ip:      bannedIp,
			BanEnds: banEnds,
		}.Serialize())
		Stats.Unlock()
	default:
		log.Err("unknown packet type", packet)
		return
	}
}

// ban master to server packet
type banM2S struct {
	Ip      string
	BanEnds uint64
}

func (b banM2S) Serialize() []byte {
	s := serializer.Serializer{
		Data: []byte{0}, // packet MasterToSlave id 0
	}

	s.AddString(b.Ip)
	s.AddUint64(b.BanEnds)

	return s.Data
}
