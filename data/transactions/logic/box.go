// Copyright (C) 2019-2022 Algorand, Inc.
// This file is part of go-algorand
//
// go-algorand is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as
// published by the Free Software Foundation, either version 3 of the
// License, or (at your option) any later version.
//
// go-algorand is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with go-algorand.  If not, see <https://www.gnu.org/licenses/>.

package logic

import (
	"fmt"

	"github.com/algorand/go-algorand/data/basics"
)

func opBoxCreate(cx *EvalContext) error {
	last := len(cx.stack) - 1 // name
	prev := last - 1          // size

	name := string(cx.stack[last].Bytes)
	size := cx.stack[prev].Uint

	// This is questionable! We need to think about how boxes can be made during
	// the txgroup that constructs the app.  The app won't be funded at create
	// time, but supposing someone uses the "trampoline" technique to fund it in
	// a later txn, if an even later txn invokes it, can it create any boxes?
	if !cx.availableBox(name) {
		return fmt.Errorf("invalid Box reference %v", name)
	}
	err := cx.Ledger.NewBox(cx.appID, name, size)
	if err != nil {
		return err
	}

	cx.stack = cx.stack[:prev]
	return nil
}

func (cx *EvalContext) availableBox(name string) bool {
	if available, ok := cx.available.boxes[cx.appID]; ok {
		for _, n := range available {
			if name == n {
				return true
			}
		}
	}
	return false
}

func opBoxExtract(cx *EvalContext) error {
	last := len(cx.stack) - 1 // length
	prev := last - 1          // start
	pprev := prev - 1         // name

	name := string(cx.stack[pprev].Bytes)
	start := cx.stack[prev].Uint
	length := cx.stack[last].Uint

	if !cx.availableBox(name) {
		return fmt.Errorf("invalid Box reference %v", name)
	}
	box, err := cx.Ledger.GetBox(cx.appID, name)
	if err != nil {
		return err
	}

	bytes, err := extractCarefully([]byte(box), start, length)
	cx.stack[pprev].Bytes = bytes
	cx.stack = cx.stack[:prev]
	return err
}

func opBoxReplace(cx *EvalContext) error {
	last := len(cx.stack) - 1 // replacement
	prev := last - 1          // start
	pprev := prev - 1         // name

	replacement := cx.stack[last].Bytes
	start := cx.stack[prev].Uint
	name := string(cx.stack[pprev].Bytes)

	if !cx.availableBox(name) {
		return fmt.Errorf("invalid Box reference %v", name)
	}
	box, err := cx.Ledger.GetBox(cx.appID, name)
	if err != nil {
		return err
	}

	bytes, err := replaceCarefully([]byte(box), replacement, start)
	if err != nil {
		return err
	}
	cx.stack[prev].Bytes = bytes
	cx.stack = cx.stack[:pprev]
	return cx.Ledger.SetBox(cx.appID, name, string(bytes))
}

func opBoxDel(cx *EvalContext) error {
	last := len(cx.stack) - 1 // name
	name := string(cx.stack[last].Bytes)

	if !cx.availableBox(name) {
		return fmt.Errorf("invalid Box reference %v", name)
	}
	cx.stack = cx.stack[:last]
	return cx.Ledger.DelBox(cx.appID, name)
}

// MakeBoxKey creates the key that a box named `name` under app `appIdx` should use.
func MakeBoxKey(appIdx basics.AppIndex, name string) string {
	// Reconsider this for something faster.  Maybe msgpack encoding of array
	// ["bx",appIdx,key]?
	return fmt.Sprintf("bx:%d:%s", appIdx, name)
}
