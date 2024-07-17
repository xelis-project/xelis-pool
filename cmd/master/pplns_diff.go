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

import "xelpool/cfg"

// returns the current PPLNS window duration in seconds
// Stats MUST be Rlocked before calling this
func GetPplnsWindow() uint64 {
	blockFoundInterval := Stats.NetHashrate / Stats.PoolHashrate * float64(cfg.Cfg.BlockTime)

	if blockFoundInterval == 0 {
		return 2 * 3600 * 24
	}

	// PPLNS window is at most 1 hour
	if blockFoundInterval > 1*3600 {
		return 1 * 3600
	}

	// PPLNS window is at most twice the block time
	if blockFoundInterval < float64(cfg.Cfg.BlockTime)*2 {
		return cfg.Cfg.BlockTime * 2
	}

	return uint64(blockFoundInterval)
}
