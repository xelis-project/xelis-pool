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

package stratum

import "encoding/json"

type RequestIn struct {
	Id     uint32          `json:"id"`
	Method string          `json:"method"`
	Params json.RawMessage `json:"params"`
}
type RequestOut struct {
	Id     uint32 `json:"id"`
	Method string `json:"method"`
	Params any    `json:"params,omitempty"`
}

type ResponseIn struct {
	Id     uint32 `json:"id"`
	Result any    `json:"result"`
	Error  *Error `json:"error,omitempty"`
}
type ResponseOut struct {
	Id     uint32 `json:"id"`
	Result any    `json:"result"`
	Error  *Error `json:"error,omitempty"`
}

type Error struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}
