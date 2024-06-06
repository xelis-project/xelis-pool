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

package main

import (
	"time"
	"xelpool/log"

	"github.com/disgoorg/disgo/discord"
	"github.com/xelis-project/xelis-go-sdk/daemon"
)

func OnBlockFound(hash string) {
	bl, err := newDaemonRPC().GetBlockByHash(daemon.GetBlockByHashParams{
		Hash:       hash,
		IncludeTxs: false,
	})
	if err != nil {
		log.Err(err)
	}
	if bl.MinerReward == nil {
		log.Err("miner reward is nil")
		z := uint64(0)
		bl.MinerReward = &z
	}

	Stats.Lock()
	defer Stats.Unlock()

	Stats.LastBlock = LastBlock{
		Height:    bl.Height,
		Timestamp: time.Now().Unix(),
		Reward:    *bl.MinerReward,
		Hash:      hash,
	}
	effort := Stats.Hashes / Stats.Difficulty * 32
	Stats.BlocksFound = append([]FoundInfo{{
		Height: bl.Height,
		Hash:   hash,
		Effort: float32(effort),
		Time:   uint64(time.Now().Unix()),
	}}, Stats.BlocksFound...)
	Stats.NumFound++
	Stats.Hashes = 0

	Stats.Cleanup()

	if discordWebhook != nil {
		_, err = discordWebhook.CreateEmbeds([]discord.Embed{discord.NewEmbedBuilder().
			SetTitlef("Block found at height %d", bl.Height).
			SetDescriptionf("Hash: %s\nEffort: %f %%", hash, effort*100).
			Build(),
		})
	}

	if err != nil {
		log.Warn(err)
		return
	}

	log.Info("webhook submit successfully")

}
