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

package sync

import (
	"xelpool/log"

	sync "github.com/sasha-s/go-deadlock"
)

type RWMutex struct {
	mutex sync.RWMutex
}

var numLock sync.RWMutex
var numLocked int
var numRLocked int

func (r *RWMutex) Lock() {
	if log.LogLevel > 2 {
		numLock.Lock()
		numLocked++
		log.Mutex("Lock!", numLocked)
		numLock.Unlock()
	}
	r.mutex.Lock()
}

func (r *RWMutex) Unlock() {
	if log.LogLevel > 2 {
		numLock.Lock()
		numLocked--
		log.Mutex("Unlock!", numLocked)
		numLock.Unlock()
	}

	r.mutex.Unlock()
}

func (r *RWMutex) RLock() {
	if log.LogLevel > 2 {
		numLock.Lock()
		numRLocked++
		log.Mutex("RLock!", numRLocked)
		numLock.Unlock()
	}
	r.mutex.RLock()
}

func (r *RWMutex) RUnlock() {
	if log.LogLevel > 2 {
		numLock.Lock()
		numRLocked--
		log.Mutex("RUnlock!", numRLocked)
		numLock.Unlock()
	}

	r.mutex.RUnlock()
}
