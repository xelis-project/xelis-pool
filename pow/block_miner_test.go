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

package pow

import (
	"encoding/base64"
	"testing"
)

const TEST_TIMESTAMP = 0x6553600123456789

func TestBlockMiner(t *testing.T) {

	bl := NewBlockMiner([32]byte{0x11, 0x22, 0x33}, [32]byte{0x44, 0x55, 0x66}, [32]byte{0x77, 0x88, 0x99})

	bl.SetTimestamp(TEST_TIMESTAMP)

	t.Logf("Data: %x\n", bl)
	t.Logf("Blob: %s\n", base64.StdEncoding.EncodeToString(bl.GetBlob()))

	bl2, err := NewBlockMinerFromBlob(bl.GetBlob())
	if err != nil {
		t.Fatal(err)
	}

	bl2.SetTimestamp(TEST_TIMESTAMP)

	t.Logf("Data: %x\n", bl2)

	if bl2 != bl {
		t.Fatal("blocks do not match")
	}

	bl.SetNonce(bl.GetNonce())
	bl.SetTimestamp(bl.GetTimestamp())

	t.Logf("Hash: %x", bl.Hash())

	var expected = [32]byte{212, 43, 173, 95, 141, 46, 3, 75, 142, 248, 13, 200, 57, 20, 28, 122,
		124, 69, 12, 56, 16, 246, 63, 0, 138, 215, 121, 34, 93, 202, 173, 175}

	if bl.Hash() != expected {
		t.Fatalf("expected: %x; got: %x", expected, bl.Hash())
	}

}
