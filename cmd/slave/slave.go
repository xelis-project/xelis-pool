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
	"os"
	"os/signal"
	"runtime/pprof"
	"syscall"
	"time"
	"xelis-pool/cfg"
	"xelis-pool/config"
	"xelis-pool/log"
	"xelis-pool/slave"
	"xelis-pool/xatum"
	"xelis-pool/xatum/server"
)

const PPROF = false

func main() {

	if PPROF {
		f, perr := os.Create("tmp.pprof")
		if perr != nil {
			log.Fatal(perr)
		}
		pprof.StartCPUProfile(f)
		defer pprof.StopCPUProfile()

		c := make(chan os.Signal, 2)
		signal.Notify(c, os.Interrupt, syscall.SIGTERM) // subscribe to system signals
		onKill := func(c chan os.Signal) {
			_, ok := <-c

			log.Debug("onKill ok:", ok)

			pprof.StopCPUProfile()
			f.Close()
			data, err := os.ReadFile("tmp.pprof")
			if err != nil {
				log.Fatal(err)
			}
			err = os.WriteFile("cpu.pprof", data, 0o666)
			if err != nil {
				log.Fatal(err)
			}
			err = os.Remove("tmp.pprof")
			if err != nil {
				log.Fatal(err)
			}
			os.Exit(0)
		}

		// try to handle OS interrupt (SIGTERM)
		go onKill(c)
	}

	s := &server.Server{}
	sGw := &GetworkServer{}
	strat := &StratumServer{}

	go sGw.listenGetwork()
	go handleXatumConns(s)
	go func() {
		time.Sleep(10 * time.Millisecond)
		handleStratumConns(strat)
	}()

	go handleDaemon(s, sGw, strat)
	go slave.StartSlaveClient()
	go statsSender(s, sGw, strat)

	s.Start(cfg.Cfg.Slave.XatumPort)
}

func statsSender(s *server.Server, gws *GetworkServer, strat *StratumServer) {
	for {
		time.Sleep(10 * time.Second)
		s.RLock()
		gws.RLock()
		strat.RLock()
		slave.SendStats(len(s.Connections)+len(strat.Conns), len(gws.Conns))
		strat.RUnlock()
		gws.RUnlock()
		s.RUnlock()
	}
}

func sendPingPackets(s *server.Server, conn *server.Connection) {
	for {
		time.Sleep((config.SLAVE_MINER_TIMEOUT - 5) * time.Second)

		err := conn.Send(xatum.PacketS2C_Ping, map[string]any{})
		if err != nil {
			log.Debug(err)
			s.Kick(conn.Id)
			return
		}
	}
}
