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

package bech32

// ChecksumConst is a type that represents the currently defined bech32
// checksum constants.
type ChecksumConst int

const (
	// Version0Const is the original constant used in the checksum
	// verification for bech32.
	Version0Const ChecksumConst = 1

	// VersionMConst is the new constant used for bech32m checksum
	// verification.
	VersionMConst ChecksumConst = 0x2bc830a3
)

// Version defines the current set of bech32 versions.
type Version uint8

const (
	// Version0 defines the original bech version.
	Version0 Version = iota

	// VersionM is the new bech32 version defined in BIP-350, also known as
	// bech32m.
	VersionM

	// VersionUnknown denotes an unknown bech version.
	VersionUnknown
)

// VersionToConsts maps bech32 versions to the checksum constant to be used
// when encoding, and asserting a particular version when decoding.
var VersionToConsts = map[Version]ChecksumConst{
	Version0: Version0Const,
	VersionM: VersionMConst,
}

// ConstsToVersion maps a bech32 constant to the version it's associated with.
var ConstsToVersion = map[ChecksumConst]Version{
	Version0Const: Version0,
	VersionMConst: VersionM,
}
