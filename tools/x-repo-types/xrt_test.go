// Copyright (C) 2019-2023 Algorand, Inc.
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

package main

import (
	"testing"

	"github.com/algorand/go-algorand/test/partitiontest"
)

type testCase struct {
	name                                       string
	xPkg, xBranch, xType, yPkg, yBranch, yType string
	expectedErr                                error
	skip                                       bool
	skipReason                                 string
}

func TestRunApp(t *testing.T) {
	partitiontest.PartitionTest(t)

	testCases := []testCase{
		{
			name:        "SDK: StateDelta",
			xPkg:        "github.com/algorand/go-algorand/ledger/ledgercore",
			xBranch:     "",
			xType:       "StateDelta",
			yPkg:        "github.com/algorand/go-algorand-sdk/v2/types",
			yBranch:     "develop",
			yType:       "LedgerStateDelta",
			expectedErr: nil,
		},
		{
			name:        "goal-v-sdk-genesis",
			xPkg:        "github.com/algorand/go-algorand/data/bookkeeping",
			xType:       "Genesis",
			yPkg:        "github.com/algorand/go-algorand-sdk/v2/types",
			yBranch:     "develop",
			yType:       "Genesis",
			expectedErr: nil,
			skip:        true,
			skipReason:  `LEVEL 3 goal basics.AccountData has 12 fields missing from SDK types.Account`,
		},
		{
			name:        "goal-v-sdk-block",
			xPkg:        "github.com/algorand/go-algorand/data/bookkeeping",
			xType:       "Block",
			yPkg:        "github.com/algorand/go-algorand-sdk/v2/types",
			yBranch:     "develop",
			yType:       "Block",
			expectedErr: nil,
			skip:        true,
			skipReason:  `LEVEL 3 goal transactions.EvalDelta has [SharedAccts](codec:"sa,allocbound=config.MaxEvalDeltaAccounts") VS SDK types.EvalDelta missing`,
		},
		{
			name:        "goal-v-sdk-blockheader",
			xPkg:        "github.com/algorand/go-algorand/data/bookkeeping",
			xType:       "BlockHeader",
			yPkg:        "github.com/algorand/go-algorand-sdk/v2/types",
			yBranch:     "develop",
			yType:       "BlockHeader",
			expectedErr: nil,
		},
		{
			name:        "goal-v-sdk-stateproof",
			xPkg:        "github.com/algorand/go-algorand/crypto/stateproof",
			xType:       "StateProof",
			yPkg:        "github.com/algorand/go-algorand-sdk/v2/types",
			yBranch:     "develop",
			yType:       "StateProof",
			expectedErr: nil,
		},
		{
			name:        "goal-v-spv-stateproof",
			xPkg:        "github.com/algorand/go-algorand/crypto/stateproof",
			xType:       "StateProof",
			yPkg:        "github.com/algorand/go-stateproof-verification/stateproof",
			yType:       "StateProof",
			expectedErr: nil,
		},
	}

	for _, tc := range testCases {
		// These should be run in serial as they modify typeAnalyzer/main.go
		// TODO: it probably is preferrable to `go get` everything _before_ running the tests.
		t.Run(tc.name, func(t *testing.T) {
			if tc.skip {
				t.Skip(tc.skipReason)
			}
			err := runApp(tc.xPkg, tc.xBranch, tc.xType, tc.yPkg, tc.yBranch, tc.yType)
			if err != tc.expectedErr {
				t.Errorf("Expected error: %v, got: %v", tc.expectedErr, err)
			}
		})
	}
}
