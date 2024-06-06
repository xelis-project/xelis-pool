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

package slave

import (
	"crypto/rand"
	"encoding/binary"
	"io"
	"net"
	"time"
	"xelpool/cfg"
	"xelpool/log"
	"xelpool/mut"
	"xelpool/serializer"

	"golang.org/x/crypto/chacha20poly1305"
)

var conn net.Conn
var connMut mut.RWMutex

const Overhead = 40

var OnBan func(ip string, ends int64)

func StartSlaveClient() {
out:
	for {
		log.Info("Connecting to master server:", cfg.Cfg.Slave.MasterAddress)

		var err error
		conn, err = net.Dial("tcp", cfg.Cfg.Slave.MasterAddress)

		if err != nil {
			log.Err(err)
			time.Sleep(time.Second)
			continue out
		}

		for {
			lenBuf := make([]byte, 2+Overhead)
			_, err := io.ReadFull(conn, lenBuf)
			if err != nil {
				log.Warn(err)
				conn.Close()
				time.Sleep(time.Second)
				continue out
			}
			lenBuf, err = Decrypt(lenBuf)
			if err != nil {
				log.Warn(err)
				conn.Close()
				time.Sleep(time.Second)
				continue out
			}
			len := int(lenBuf[0]) | (int(lenBuf[1]) << 8)

			// read the actual message

			buf := make([]byte, len+Overhead)
			_, err = io.ReadFull(conn, buf)
			if err != nil {
				log.Warn(err)
				conn.Close()
				time.Sleep(time.Second)
				continue out
			}
			buf, err = Decrypt(buf)
			if err != nil {
				log.Warn(err)
				conn.Close()
				time.Sleep(time.Second)
				continue out
			}
			log.Netf("Received message: %x", buf)
			OnMessage(buf)
		}

	}
}

func OnMessage(b []byte) {
	d := serializer.Deserializer{
		Data: b,
	}

	packet := d.ReadUint8()

	switch packet {
	case 0: // BanM2S
		ip := d.ReadString()
		banEnds := d.ReadUint64()

		if d.Error != nil {
			log.Warn(d.Error)
			return
		}
		log.Infof("received ban from master, ip: %s ends: %d", ip, banEnds)

		// OnBan(ip, int64(banEnds))
	}
}

func SendShare(wallet string, diff uint64) {
	connMut.Lock()
	defer connMut.Unlock()

	cacheShare(wallet, diff)
}

func SendBlockFound(hash [32]byte) {
	s := serializer.Serializer{
		Data: []byte{1},
	}

	s.AddFixedByteArray(hash[:], 32)

	// wait 5 seconds to avoid sending "block found" before the daemon knows it
	go func() {
		time.Sleep(5 * time.Second)
		sendToConn(s.Data)
	}()
}
func SendStats(nrMiners, nrGetworkMiners int) {
	s := serializer.Serializer{
		Data: []byte{2},
	}
	s.AddUvarint(uint64(nrMiners) + uint64(nrGetworkMiners))

	sendToConn(s.Data)
}
func SendBan(ip string, ends int64) {
	s := serializer.Serializer{
		Data: []byte{4},
	}

	s.AddString(ip)
	s.AddUint64(uint64(ends))

	sendToConn(s.Data)
}

func sendToConn(data []byte) {
	if conn == nil {
		log.Err("SendToConn: Connection is nil")
		return
	}
	var dataLenBin = make([]byte, 0, 2)
	dataLenBin = binary.LittleEndian.AppendUint16(dataLenBin, uint16(len(data)))
	conn.Write(Encrypt(dataLenBin))
	conn.Write(Encrypt(data))
}

func Encrypt(msg []byte) []byte {
	aead, err := chacha20poly1305.NewX(cfg.MasterPass[:])
	if err != nil {
		panic(err)
	}

	nonce := make([]byte, aead.NonceSize(), aead.NonceSize()+len(msg)+aead.Overhead())
	rand.Read(nonce)

	// Encrypt the message and append the ciphertext to the nonce.
	return aead.Seal(nonce, nonce, msg, nil)
}

func Decrypt(msg []byte) ([]byte, error) {
	aead, err := chacha20poly1305.NewX(cfg.MasterPass[:])
	if err != nil {
		panic(err)
	}

	if len(msg) < aead.NonceSize() {
		panic("ciphertext too short")
	}

	// Split nonce and ciphertext.
	nonce, ciphertext := msg[:aead.NonceSize()], msg[aead.NonceSize():]

	// Decrypt the message and check it wasn't tampered with.
	decrypted, err := aead.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return []byte{}, err
	}

	return decrypted, nil
}
