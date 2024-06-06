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

package ratelimit

import (
	"sync"
	"time"
	"xelpool/log"
)

// max score of 2000 per minute

/*
consumption:
connect: 50 (20 per minute)
share submit: 25 (40 per minute)
invalid share PoW: 200 (5 per minute)
*/

const (
	ACTION_CONNECT           = 10
	ACTION_SHARE_SUBMIT      = 1
	ACTION_INVALID_SHARE_POW = 200
)

const MAX_SCORE = 2000
const RESET_INTERVAL = 120 * time.Second
const BAN_DURATION = 5 * 60

// rate limiters per IP
var rlMut sync.RWMutex
var rateLimiters = make(map[string]rateLimiter, 500)
var bans = make(map[string]ban, 10)

type rateLimiter struct {
	Score uint32
}
type ban struct {
	Ends int64
}

func Ban(ip string, ends int64) {
	rlMut.Lock()
	defer rlMut.Unlock()

	bans[ip] = ban{
		Ends: time.Now().Unix() + BAN_DURATION,
	}
}

func CanDoAction(ip string, requiredScore uint32) bool {
	rlMut.Lock()
	defer rlMut.Unlock()

	log.Debug("rate limit score", rateLimiters[ip].Score, "/", MAX_SCORE)

	rateLimiters[ip] = rateLimiter{
		Score: rateLimiters[ip].Score + requiredScore,
	}

	t := time.Now().Unix()

	if bans[ip].Ends > t {
		return false
	}

	if rateLimiters[ip].Score > MAX_SCORE {
		bans[ip] = ban{
			Ends: t + BAN_DURATION,
		}
		//go slave.SendBan(ip, t+BAN_DURATION)
		return false
	}

	return true
}

// periodically clear rate limiters
func init() {
	go func() {
		for {
			time.Sleep(RESET_INTERVAL)
			clearRl()
		}
	}()
}

func clearRl() {
	rlMut.Lock()
	defer rlMut.Unlock()

	// clear rate limiters
	rateLimiters = make(map[string]rateLimiter, len(rateLimiters))

	// clear outdated bans
	t := time.Now().Unix()
	bans2 := make(map[string]ban, len(bans))
	for i, v := range bans {
		if v.Ends > t { // ban is not outdated
			bans2[i] = v
		}
	}
	bans = bans2
}
