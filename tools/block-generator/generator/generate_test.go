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

package generator

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"text/template"
	"time"

	"github.com/algorand/go-algorand/crypto"
	"github.com/algorand/go-algorand/data/basics"
	"github.com/algorand/go-algorand/data/bookkeeping"
	"github.com/algorand/go-algorand/data/transactions"
	"github.com/algorand/go-algorand/data/transactions/logic"
	"github.com/algorand/go-algorand/protocol"
	"github.com/algorand/go-algorand/rpcs"
	"github.com/algorand/go-algorand/test/partitiontest"
	"github.com/stretchr/testify/require"
)

func makePrivateGenerator(t *testing.T, round uint64, genesis bookkeeping.Genesis) *generator {
	cfg := GenerationConfig{
		Name:                         "test",
		NumGenesisAccounts:           10,
		GenesisAccountInitialBalance: 1000000000000,
		PaymentTransactionFraction:   1.0,
		PaymentNewAccountFraction:    1.0,
		AssetCreateFraction:          1.0,
	}
	cfg.validateWithDefaults(true)
	publicGenerator, err := MakeGenerator(round, genesis, cfg, true)
	require.NoError(t, err)
	return publicGenerator.(*generator)
}

func TestPaymentAcctCreate(t *testing.T) {
	partitiontest.PartitionTest(t)
	g := makePrivateGenerator(t, 0, bookkeeping.Genesis{})
	g.generatePaymentTxnInternal(paymentAcctCreateTx, 0, 0)
	require.Len(t, g.balances, int(g.config.NumGenesisAccounts+1))
}

func TestPaymentTransfer(t *testing.T) {
	partitiontest.PartitionTest(t)
	g := makePrivateGenerator(t, 0, bookkeeping.Genesis{})
	g.generatePaymentTxnInternal(paymentTx, 0, 0)
	require.Len(t, g.balances, int(g.config.NumGenesisAccounts))
}

func TestAssetXferNoAssetsOverride(t *testing.T) {
	partitiontest.PartitionTest(t)
	g := makePrivateGenerator(t, 0, bookkeeping.Genesis{})

	// First asset transaction must create.
	actual, txn, assetID := g.generateAssetTxnInternal(assetXfer, 1, 0)
	require.NotEqual(t, 0, assetID)
	require.Equal(t, assetCreate, actual)
	require.Equal(t, protocol.AssetConfigTx, txn.Type)
	require.Len(t, g.assets, 0)
	require.Len(t, g.pendingAssets, 1)
	require.Len(t, g.pendingAssets[0].holdings, 1)
	require.Len(t, g.pendingAssets[0].holders, 1)
}

func TestAssetXferOneHolderOverride(t *testing.T) {
	partitiontest.PartitionTest(t)
	g := makePrivateGenerator(t, 0, bookkeeping.Genesis{})
	g.finishRound()
	g.generateAssetTxnInternal(assetCreate, 1, 0)
	g.finishRound()

	// Transfer converted to optin if there is only 1 holder.
	actual, txn, assetID := g.generateAssetTxnInternal(assetXfer, 2, 0)
	require.NotEqual(t, 0, assetID)
	require.Equal(t, assetOptin, actual)
	require.Equal(t, protocol.AssetTransferTx, txn.Type)
	require.Len(t, g.assets, 1)
	// A new holding is created, indicating the optin
	require.Len(t, g.assets[0].holdings, 2)
	require.Len(t, g.assets[0].holders, 2)
}

func TestAssetCloseCreatorOverride(t *testing.T) {
	partitiontest.PartitionTest(t)
	g := makePrivateGenerator(t, 0, bookkeeping.Genesis{})
	g.finishRound()
	g.generateAssetTxnInternal(assetCreate, 1, 0)
	g.finishRound()

	// Instead of closing the creator, optin a new account
	actual, txn, assetID := g.generateAssetTxnInternal(assetClose, 2, 0)
	require.NotEqual(t, 0, assetID)
	require.Equal(t, assetOptin, actual)
	require.Equal(t, protocol.AssetTransferTx, txn.Type)
	require.Len(t, g.assets, 1)
	// A new holding is created, indicating the optin
	require.Len(t, g.assets[0].holdings, 2)
	require.Len(t, g.assets[0].holders, 2)
}

func TestAssetOptinEveryAccountOverride(t *testing.T) {
	partitiontest.PartitionTest(t)
	g := makePrivateGenerator(t, 0, bookkeeping.Genesis{})
	g.finishRound()
	g.generateAssetTxnInternal(assetCreate, 1, 0)
	g.finishRound()

	// Opt all the accounts in, this also verifies that no account is opted in twice
	var txn transactions.Transaction
	var actual TxTypeID
	var assetID uint64
	for i := 2; uint64(i) <= g.numAccounts; i++ {
		actual, txn, assetID = g.generateAssetTxnInternal(assetOptin, 2, uint64(1+i))
		require.NotEqual(t, 0, assetID)
		require.Equal(t, assetOptin, actual)
		require.Equal(t, protocol.AssetTransferTx, txn.Type)
		require.Len(t, g.assets, 1)
		require.Len(t, g.assets[0].holdings, i)
		require.Len(t, g.assets[0].holders, i)
	}
	g.finishRound()

	// All accounts have opted in
	require.Equal(t, g.numAccounts, uint64(len(g.assets[0].holdings)))

	// The next optin closes instead
	actual, txn, assetID = g.generateAssetTxnInternal(assetOptin, 3, 0)
	require.Greater(t, assetID, uint64(0))
	g.finishRound()
	require.Equal(t, assetClose, actual)
	require.Equal(t, protocol.AssetTransferTx, txn.Type)
	require.Len(t, g.assets, 1)
	require.Len(t, g.assets[0].holdings, int(g.numAccounts-1))
	require.Len(t, g.assets[0].holders, int(g.numAccounts-1))
}

func TestAssetDestroyWithHoldingsOverride(t *testing.T) {
	partitiontest.PartitionTest(t)
	g := makePrivateGenerator(t, 0, bookkeeping.Genesis{})
	g.finishRound()
	g.generateAssetTxnInternal(assetCreate, 1, 0)
	g.finishRound()
	g.generateAssetTxnInternal(assetOptin, 2, 0)
	g.finishRound()
	g.generateAssetTxnInternal(assetXfer, 3, 0)
	g.finishRound()
	require.Len(t, g.assets[0].holdings, 2)
	require.Len(t, g.assets[0].holders, 2)

	actual, txn, assetID := g.generateAssetTxnInternal(assetDestroy, 4, 0)
	require.NotEqual(t, 0, assetID)
	require.Equal(t, assetClose, actual)
	require.Equal(t, protocol.AssetTransferTx, txn.Type)
	require.Len(t, g.assets, 1)
	require.Len(t, g.assets[0].holdings, 1)
	require.Len(t, g.assets[0].holders, 1)
}

func TestAssetTransfer(t *testing.T) {
	partitiontest.PartitionTest(t)
	g := makePrivateGenerator(t, 0, bookkeeping.Genesis{})
	g.finishRound()

	g.generateAssetTxnInternal(assetCreate, 1, 0)
	g.finishRound()
	g.generateAssetTxnInternal(assetOptin, 2, 0)
	g.finishRound()
	g.generateAssetTxnInternal(assetXfer, 3, 0)
	g.finishRound()
	require.NotEqual(t, g.assets[0].holdings[1].balance, uint64(0))
}

func TestAssetDestroy(t *testing.T) {
	partitiontest.PartitionTest(t)
	g := makePrivateGenerator(t, 0, bookkeeping.Genesis{})
	g.finishRound()
	g.generateAssetTxnInternal(assetCreate, 1, 0)
	g.finishRound()
	require.Len(t, g.assets, 1)

	actual, txn, assetID := g.generateAssetTxnInternal(assetDestroy, 2, 0)
	require.NotEqual(t, 0, assetID)
	require.Equal(t, assetDestroy, actual)
	require.Equal(t, protocol.AssetConfigTx, txn.Type)
	require.Len(t, g.assets, 0)
}

type assembledPrograms struct {
	boxesApproval     []byte
	boxesClear        []byte
	swapOuterApproval []byte
	swapOuterClear    []byte
	swapInnerApproval []byte
	swapInnerClear    []byte
}

func assembleApps(t *testing.T, assetIDs ...uint64) assembledPrograms {
	t.Helper()

	assembled := make([][]byte, 5)
	for i, teal := range []string{
		approvalBoxes,
		clearBoxes,
		approvalSwapOuter,
		clearSwapOuter,
		clearSwapInner,
	} {
		ops, err := logic.AssembleString(teal)
		require.NoError(t, err, fmt.Sprintf("failed to assemble TEAL @ index %d", i))
		assembled[i] = ops.Program
	}
	programs := assembledPrograms{
		boxesApproval:     assembled[0],
		boxesClear:        assembled[1],
		swapOuterApproval: assembled[2],
		swapOuterClear:    assembled[3],
		swapInnerClear:    assembled[4],
	}

	if len(assetIDs) > 0 {
		require.Len(t, assetIDs, 2)
		templateIDs := struct {
			AssetID1 uint64
			AssetID2 uint64
		}{assetIDs[0], assetIDs[1]}

		tmpl, err := template.New("tealSwapInner").Parse(approvalSwapInnerTemplate)
		require.NoError(t, err)

		var tealBuf bytes.Buffer
		err = tmpl.Execute(&tealBuf, templateIDs)
		require.NoError(t, err)

		ops, err := logic.AssembleString(tealBuf.String())
		require.NoError(t, err)
		programs.swapInnerApproval = ops.Program
	}
	return programs
}

func TestAppCreate(t *testing.T) {
	partitiontest.PartitionTest(t)
	t.Parallel()

	g := makePrivateGenerator(t, 0, bookkeeping.Genesis{})
	assembled := assembleApps(t)

	round, intra := uint64(1337), uint64(0)
	hint := appData{sender: 7}

	// app call transaction creating appBoxes
	actual, sgnTxns, appID, err := g.generateAppCallInternal(appBoxesCreate, round, intra, &hint)
	require.Greater(t, appID, uint64(0))
	require.NoError(t, err)
	require.Equal(t, appBoxesCreate, actual)

	require.Len(t, sgnTxns, 2)
	createTxn := sgnTxns[0].Txn

	require.Equal(t, indexToAccount(hint.sender), createTxn.Sender)
	require.Equal(t, protocol.ApplicationCallTx, createTxn.Type)
	require.Equal(t, basics.AppIndex(0), createTxn.ApplicationCallTxnFields.ApplicationID)
	require.Equal(t, assembled.boxesApproval, createTxn.ApplicationCallTxnFields.ApprovalProgram)
	require.Equal(t, assembled.boxesClear, createTxn.ApplicationCallTxnFields.ClearStateProgram)
	require.Equal(t, uint64(32), createTxn.ApplicationCallTxnFields.GlobalStateSchema.NumByteSlice)
	require.Equal(t, uint64(32), createTxn.ApplicationCallTxnFields.GlobalStateSchema.NumUint)
	require.Equal(t, uint64(8), createTxn.ApplicationCallTxnFields.LocalStateSchema.NumByteSlice)
	require.Equal(t, uint64(8), createTxn.ApplicationCallTxnFields.LocalStateSchema.NumUint)
	require.Equal(t, transactions.NoOpOC, createTxn.ApplicationCallTxnFields.OnCompletion)

	require.Len(t, g.pendingAppSlice[appKindBoxes], 1)
	require.Len(t, g.pendingAppSlice[appKindSwapOuter], 0)
	require.Len(t, g.pendingAppMap[appKindBoxes], 1)
	require.Len(t, g.pendingAppMap[appKindSwapOuter], 0)
	ad := g.pendingAppSlice[appKindBoxes][0]
	require.Equal(t, ad, g.pendingAppMap[appKindBoxes][ad.appID])
	require.Equal(t, hint.sender, ad.sender)
	require.Equal(t, appKindBoxes, ad.kind)
	optins := ad.optins
	require.Len(t, optins, 0)

	paySiblingTxn := sgnTxns[1].Txn
	require.Equal(t, protocol.PaymentTx, paySiblingTxn.Type)

	// app call transaction creating appSwapOuter
	intra = 1
	actual, sgnTxns, appID, err = g.generateAppCallInternal(appSwapOuterCreate, round, intra, &hint)
	require.Greater(t, appID, uint64(0))
	require.NoError(t, err)
	require.Equal(t, appSwapOuterCreate, actual)

	require.Len(t, sgnTxns, 1)
	createTxn = sgnTxns[0].Txn

	require.Equal(t, protocol.ApplicationCallTx, createTxn.Type)
	require.Equal(t, indexToAccount(hint.sender), createTxn.Sender)
	require.Equal(t, basics.AppIndex(0), createTxn.ApplicationCallTxnFields.ApplicationID)
	require.Equal(t, assembled.swapOuterApproval, createTxn.ApplicationCallTxnFields.ApprovalProgram)
	require.Equal(t, assembled.swapOuterClear, createTxn.ApplicationCallTxnFields.ClearStateProgram)
	require.Equal(t, uint64(32), createTxn.ApplicationCallTxnFields.GlobalStateSchema.NumByteSlice)
	require.Equal(t, uint64(32), createTxn.ApplicationCallTxnFields.GlobalStateSchema.NumUint)
	require.Equal(t, uint64(8), createTxn.ApplicationCallTxnFields.LocalStateSchema.NumByteSlice)
	require.Equal(t, uint64(8), createTxn.ApplicationCallTxnFields.LocalStateSchema.NumUint)
	require.Equal(t, transactions.NoOpOC, createTxn.ApplicationCallTxnFields.OnCompletion)

	require.Len(t, g.pendingAppSlice[appKindBoxes], 1)
	require.Len(t, g.pendingAppSlice[appKindSwapOuter], 1)
	require.Len(t, g.pendingAppMap[appKindBoxes], 1)
	require.Len(t, g.pendingAppMap[appKindSwapOuter], 1)
	ad = g.pendingAppSlice[appKindSwapOuter][0]
	require.Equal(t, ad, g.pendingAppMap[appKindSwapOuter][ad.appID])
	require.Equal(t, hint.sender, ad.sender)
	require.Equal(t, appKindSwapOuter, ad.kind)
	optins = ad.optins
	require.Len(t, optins, 0)
}

func TestAppBoxesOptin(t *testing.T) {
	partitiontest.PartitionTest(t)
	t.Parallel()

	g := makePrivateGenerator(t, 0, bookkeeping.Genesis{})
	assembled := assembleApps(t)

	round, intra := uint64(1337), uint64(0)

	hint := appData{sender: 7}

	// app call transaction opting into boxes gets replaced by creating appBoxes
	actual, sgnTxns, appID, err := g.generateAppCallInternal(appBoxesOptin, round, intra, &hint)
	require.Greater(t, appID, uint64(0))
	require.NoError(t, err)
	require.Equal(t, appBoxesCreate, actual)

	require.Len(t, sgnTxns, 2)
	createTxn := sgnTxns[0].Txn

	require.Equal(t, protocol.ApplicationCallTx, createTxn.Type)
	require.Equal(t, indexToAccount(hint.sender), createTxn.Sender)
	require.Equal(t, basics.AppIndex(0), createTxn.ApplicationCallTxnFields.ApplicationID)
	require.Equal(t, assembled.boxesApproval, createTxn.ApplicationCallTxnFields.ApprovalProgram)
	require.Equal(t, assembled.boxesClear, createTxn.ApplicationCallTxnFields.ClearStateProgram)
	require.Equal(t, uint64(32), createTxn.ApplicationCallTxnFields.GlobalStateSchema.NumByteSlice)
	require.Equal(t, uint64(32), createTxn.ApplicationCallTxnFields.GlobalStateSchema.NumUint)
	require.Equal(t, uint64(8), createTxn.ApplicationCallTxnFields.LocalStateSchema.NumByteSlice)
	require.Equal(t, uint64(8), createTxn.ApplicationCallTxnFields.LocalStateSchema.NumUint)
	require.Equal(t, transactions.NoOpOC, createTxn.ApplicationCallTxnFields.OnCompletion)
	require.Nil(t, createTxn.ApplicationCallTxnFields.Boxes)

	require.Len(t, g.pendingAppSlice[appKindBoxes], 1)
	require.Len(t, g.pendingAppSlice[appKindSwapOuter], 0)
	require.Len(t, g.pendingAppMap[appKindBoxes], 1)
	require.Len(t, g.pendingAppMap[appKindSwapOuter], 0)
	ad := g.pendingAppSlice[appKindBoxes][0]
	require.Equal(t, ad, g.pendingAppMap[appKindBoxes][ad.appID])
	require.Equal(t, hint.sender, ad.sender)
	require.Equal(t, appKindBoxes, ad.kind)
	require.Len(t, ad.optins, 0)

	require.Contains(t, effects, actual)

	paySiblingTxn := sgnTxns[1].Txn
	require.Equal(t, protocol.PaymentTx, paySiblingTxn.Type)

	// 2nd attempt to optin (with new sender) doesn't get replaced
	g.finishRound()
	intra += 1
	hint.sender = 8

	actual, sgnTxns, appID, err = g.generateAppCallInternal(appBoxesOptin, round, intra, &hint)
	require.Greater(t, appID, uint64(0))
	require.NoError(t, err)
	require.Equal(t, appBoxesOptin, actual)

	require.Len(t, sgnTxns, 2)
	pay := sgnTxns[1].Txn
	require.Equal(t, protocol.PaymentTx, pay.Type)
	require.NotEqual(t, basics.Address{}.String(), pay.Sender.String())

	createTxn = sgnTxns[0].Txn
	require.Equal(t, protocol.ApplicationCallTx, createTxn.Type)
	require.Equal(t, indexToAccount(hint.sender), createTxn.Sender)
	require.Equal(t, basics.AppIndex(1001), createTxn.ApplicationCallTxnFields.ApplicationID)
	require.Equal(t, []byte(nil), createTxn.ApplicationCallTxnFields.ApprovalProgram)
	require.Equal(t, []byte(nil), createTxn.ApplicationCallTxnFields.ClearStateProgram)
	require.Equal(t, basics.StateSchema{}, createTxn.ApplicationCallTxnFields.GlobalStateSchema)
	require.Equal(t, basics.StateSchema{}, createTxn.ApplicationCallTxnFields.LocalStateSchema)
	require.Equal(t, transactions.OptInOC, createTxn.ApplicationCallTxnFields.OnCompletion)
	require.Len(t, createTxn.ApplicationCallTxnFields.Boxes, 1)
	require.Equal(t, crypto.Digest(pay.Sender).ToSlice(), createTxn.ApplicationCallTxnFields.Boxes[0].Name)

	require.Len(t, g.pendingAppSlice[appKindBoxes], 1)
	require.Len(t, g.pendingAppSlice[appKindSwapOuter], 0)
	require.Len(t, g.pendingAppMap[appKindBoxes], 1)
	require.Len(t, g.pendingAppMap[appKindSwapOuter], 0)
	ad = g.pendingAppSlice[appKindBoxes][0]
	require.Equal(t, ad, g.pendingAppMap[appKindBoxes][ad.appID])
	require.Equal(t, hint.sender, ad.sender) // NOT 8!!!
	require.Equal(t, appKindBoxes, ad.kind)
	optins := ad.optins
	require.Len(t, optins, 1)
	require.Contains(t, optins, hint.sender)

	require.Contains(t, effects, actual)
	require.Len(t, effects[actual], 2)
	require.Equal(t, TxEffect{effectSiblingPay, 1}, effects[actual][0])
	require.Equal(t, TxEffect{effectInnerPay, 2}, effects[actual][1])

	numTxns, err := g.countAndRecordEffects(actual, time.Now())
	require.NoError(t, err)
	require.Equal(t, uint64(4), numTxns)

	// 3rd attempt to optin gets replaced by vanilla app call
	g.finishRound()
	intra += numTxns

	actual, sgnTxns, appID, err = g.generateAppCallInternal(appBoxesOptin, round, intra, &hint)
	require.Greater(t, appID, uint64(0))
	require.NoError(t, err)
	require.Equal(t, appBoxesCall, actual)

	require.Len(t, sgnTxns, 1)

	createTxn = sgnTxns[0].Txn
	require.Equal(t, protocol.ApplicationCallTx, createTxn.Type)
	require.Equal(t, indexToAccount(hint.sender), createTxn.Sender)
	require.Equal(t, basics.AppIndex(1001), createTxn.ApplicationCallTxnFields.ApplicationID)
	require.Equal(t, []byte(nil), createTxn.ApplicationCallTxnFields.ApprovalProgram)
	require.Equal(t, []byte(nil), createTxn.ApplicationCallTxnFields.ClearStateProgram)
	require.Equal(t, basics.StateSchema{}, createTxn.ApplicationCallTxnFields.GlobalStateSchema)
	require.Equal(t, basics.StateSchema{}, createTxn.ApplicationCallTxnFields.LocalStateSchema)
	require.Equal(t, transactions.NoOpOC, createTxn.ApplicationCallTxnFields.OnCompletion)
	require.Len(t, createTxn.ApplicationCallTxnFields.Boxes, 1)
	require.Equal(t, crypto.Digest(pay.Sender).ToSlice(), createTxn.ApplicationCallTxnFields.Boxes[0].Name)

	// no change to app states
	require.Len(t, g.pendingAppSlice[appKindBoxes], 0)
	require.Len(t, g.pendingAppSlice[appKindSwapOuter], 0)
	require.Len(t, g.pendingAppMap[appKindBoxes], 0)
	require.Len(t, g.pendingAppMap[appKindSwapOuter], 0)

	require.NotContains(t, effects, actual)
}

// TestAppSwap tests generating app swap transactions by repeatedly attempting
// to generate an appSwapOuterCall and going on to the next round.
/*
1. appSwapOuterCall -> assetCreate			// the first ASA in the swap-pair need exist
	- Precondition: none
	- Postcondition: g.assets has length 1 and we extract creator := g.assets[0].creator
2. appSwapOuterCall -> assetCreate  			// the second ASA in the swap-pair need exist
- Precondition: we have creator of ASA1
- Postcondition: g.assets has length 2 and g.multiCoiners has length 1 and contains creator

3. appSwapOuterCall -> appSwapInnerCreate	// the inner swap app need exist
	- Precondition: we have creator of ASA1 and ASA2
	- Postcondition:
	* g.appSlice/appMap[appKindSwapInner] have length 1
	* the relevant appData.sender is creator
	* the relevant appData.assets are ASA1 and ASA2
	* ASA1 and ASA2 are embedded in the program bytes of the create transaction
		assembled := assembleApps(t, assetID1, assetID2)

4. appSwapOuterCall -> appSwapInnerSpecialPrime // the inner swap app needs to be primed by opting into ASA1/2 and creating the LP token
  - Precondition: non-empty gen.appData[appKindSwapInner], with ASA1, AS2 := appData.assets[:2]
  - Postcondition:
    * g.appData[appKindSwapInner][0].assets == {ASA1, ASA2, ASA_LP}
	* g.assets == {ASA1, ASA2, ASA_LP}
	* g.appData[appKindSwapInner][0].assets == {ASA1, ASA2, ASA_LP}
	* ASA1.appHolders[appID].appID == ASA2.appHolders[appID].appID == appID
	* ASA_LP.creator == 0 (because not set) but ASA_LP.creator_app == appID
    * 3 top-level transactions are generated:
		. index 0: pay txn with 0.4 Algos
		. index 1: app call to the inner swap app with arg CLT
		. index 2: app call to the inner swap app with arg OPTIN and ForeignAssets == {ASA1, ASA2}
    * 2 sibling app call txns are generated
	* 1 inner acfg effect txn is generated
	* 2 inner axfer effect txns are generated

5. appSwapOuterCall -> appSwapInnerSpecialLiquidity // the creator needs to provide liquidity to inner swap app
  - Precondition:
  	* non-empty gen.appData[appKindSwapInner] with ASA1, ASA2, ASA_LP := appData.assets
	* (error condition if following violated where asset = g.assets[2]
		. g.assets[0/1].holders[creator] exist
		. g.assets[0/1].holders[creator].balance >= 500_000_000
  - Postcondition:
    * g.assets[2].holders[creator] >= 500_000_000
	* 7 top-level transactions are generated:
		. index 0: axfer/optin from C to self for ASA_LP for amt 0
		. index 1: axfer from C to appAddr(inner swap app) for ASA1 for amt 4,294.930402
		. index 2: axfer from C to appAddr(inner swap app) for ASA2 for amt 4,294.930402
		. index 3: app call to the inner swap app with arg ADDLIQ and ForeignAssets == {ASA1, ASA2, ASA_LP}
		. index 4: axfer from C to appAddr(inner swap app) for ASA1 for amt 499,995,705.069598
		. index 5: axfer from C to appAddr(inner swap app) for ASA2 for amt 499,995,705.069598
		. index 6: app call to the inner swap app with arg ADDLIQ and ForeignAssets == {ASA1, ASA2, ASA_LP}
	* 4 sibling axfer txns are generated
	* 2 sibling app call txns are generated
	* 2 inner axfer effect txns are generated

6. appSwapOuterCall -> appSwapOuterCreate	// the outer swap app need exist
  - Precondition:
	* g.assets[0/1/2].holders[creator] >= 500_000_000
	* non-empty gen.appData[appKindSwapInner] with ASA1, ASA2, ASA_LP := appData.assets
  - Postcondition:
	* g.appSlice/appMap[appKindSwapOuter] have length 1

7. appSwapOuterCall -> appSwapOuterSpecialPrime
	// the outer swap app needs to be primed by opting into ASA1/2
	// and it also needs to have a good chunk of ASA1
  - Precondition: go.appMap[appKindSwapOuter] is non-empty
  - Postcondition:
    * g.appData[appKindSwapOuter][0].assets == {ASA1, ASA2}
	* ASA1.appHolders[appID].appID == appID
	*
	* 5 top-level transactions are generated:
		. index 0: pay txn with 0.1 Algos
		. index 1: app call to the outer swap app with arg 0xaa6d419d and ForeignAssets == {ASA1}
		. index 2: pay txn with 0.1 Algos
		. index 3: app call to the outer swap app with arg 0xaa6d419d and ForeignAssets == {ASA2}
		. index 4: axfer from C to appAddr(outer swap app) ASA1 @ 1_000_000 units
	* 1 sibling pay txn is generated
	* 2 sibling app call txns are generated
	* 1 sibling axfer txn is generated
	* 0 inner txns are generated

8. appSwapOuterCall -> appSwapOuterCall			// FINALLY!!!!
  - Precondition: g.appMap[appKindSwapOuter] such that ASA1 amount > 1_000 exists
  - Postcondition: None... this is a stable condition

*/
func TestAppSwap(t *testing.T) {
	partitiontest.PartitionTest(t)
	t.Parallel()

	g := makePrivateGenerator(t, 0, bookkeeping.Genesis{})
	txnCounter := g.txnCounter
	round, intra := uint64(1337), uint64(0)
	mockAdvance := func(numTxns uint64) {
		g.finishRound()
		g.startRound()
		g.txnCounter += numTxns
		round++
	}

	// We expect the
	// following evolution in the _actual_ transactions generated when starting
	// with a completely empty generator state.
	// "Precondition" is what the call expects
	// "Postcondition" is the expected effect after a gen.finishRound() call:

	// creator := uint64(7)
	// hint := appData{sender: creator}

	/*
		1. appSwapOuterCall -> assetCreate			// the first ASA in the swap-pair need exist
		  - Precondition: none
		  - Postcondition: g.assets has length 1 and we extract creator := g.assets[0].creator
	*/
	numTxns := uint64(1)
	txnCounter += numTxns
	assetID1 := txnCounter
	actual, sgnTxns, objID, err := g.generateAppCallInternal(appSwapOuterCall, round, intra, nil)
	require.Equal(t, assetCreate, actual)
	require.Len(t, sgnTxns, 1)
	axfer1_1 := sgnTxns[0].Txn
	require.Equal(t, protocol.AssetConfigTx, axfer1_1.Type)
	require.Equal(t, assetID1, objID)
	require.NoError(t, err)
	assetInfo1 := g.pendingAssets[0]
	require.Len(t, g.pendingMultiCoiners, 0)

	mockAdvance(numTxns)
	require.Equal(t, txnCounter, g.txnCounter) // sanity check
	require.Len(t, g.assets, 1)
	require.Equal(t, assetInfo1, g.assets[0])
	require.Equal(t, assetID1, assetInfo1.assetID)
	require.Len(t, g.multiCoiners, 0)

	// creator should remain constant for the rest of this test even though no hint is supplied
	creator := assetInfo1.creator
	/*
		2. appSwapOuterCall -> assetCreate  			// the second ASA in the swap-pair need exist
		- Precondition: we have creator of ASA1
		- Postcondition: g.assets has length 2 and g.multiCoiners has length 1 and contains creator
	*/
	numTxns = 1
	txnCounter += numTxns
	assetID2 := txnCounter
	actual, sgnTxns, objID, err = g.generateAppCallInternal(appSwapOuterCall, round, intra, nil)
	require.Equal(t, assetCreate, actual)
	require.Len(t, sgnTxns, 1)
	axfer2_2 := sgnTxns[0].Txn
	require.Equal(t, protocol.AssetConfigTx, axfer2_2.Type)
	require.Equal(t, assetID2, objID)
	require.NoError(t, err)
	assetInfo2 := g.pendingAssets[1]
	pendingMultiCoins := g.pendingMultiCoiners[creator]
	require.Len(t, pendingMultiCoins, 2)
	require.Equal(t, assetInfo1, pendingMultiCoins[0])
	require.Equal(t, assetInfo2, pendingMultiCoins[1])

	mockAdvance(numTxns) // sanity check
	require.Equal(t, txnCounter, g.txnCounter)
	require.Len(t, g.assets, 2)
	require.Equal(t, creator, assetInfo2.creator)
	require.Equal(t, assetID2, assetInfo2.assetID)
	require.Len(t, g.multiCoiners, 1)
	multiCoins := g.multiCoiners[creator]
	require.Equal(t, pendingMultiCoins, multiCoins)
	/*
		3. appSwapOuterCall -> appSwapInnerCreate	// the inner swap app need exist
		  - Precondition: we have creator of ASA1 and ASA2
		  - Postcondition:
		    * g.appSlice/appMap[appKindSwapInner] have length 1
		    * the relevant appData.sender is creator
		    * the relevant appData.assets are ASA1 and ASA2
		    * ASA1 and ASA2 are embedded in the program bytes of the create transaction
				assembled := assembleApps(t, assetID1, assetID2)
	*/
	assembled := assembleApps(t, assetID1, assetID2)

	numTxns = 1
	txnCounter += uint64(numTxns)
	innerAppID := txnCounter
	actual, sgnTxns, objID, err = g.generateAppCallInternal(appSwapOuterCall, round, intra, nil)
	require.Equal(t, appSwapInnerCreate, actual)
	require.Len(t, sgnTxns, 1)
	swapInnerCreate_3 := sgnTxns[0].Txn
	require.Equal(t, protocol.ApplicationCallTx, swapInnerCreate_3.Type)
	require.Equal(t, transactions.NoOpOC, swapInnerCreate_3.ApplicationCallTxnFields.OnCompletion)
	require.Equal(t, assembled.swapInnerApproval, swapInnerCreate_3.ApplicationCallTxnFields.ApprovalProgram)
	require.Equal(t, assembled.swapInnerClear, swapInnerCreate_3.ApplicationCallTxnFields.ClearStateProgram)
	require.Equal(t, innerAppID, objID)
	require.NoError(t, err)

	mockAdvance(numTxns)
	require.Equal(t, txnCounter, g.txnCounter) // sanity check
	require.Len(t, g.appSlice[appKindSwapInner], 1)
	require.Len(t, g.appMap[appKindSwapInner], 1)
	innerAppInfo := g.appMap[appKindSwapInner][innerAppID]
	require.Equal(t, innerAppInfo, g.appSlice[appKindSwapInner][0])
	require.Equal(t, creator, innerAppInfo.sender)
	require.Len(t, innerAppInfo.assets, 2)
	require.Equal(t, assetInfo1, innerAppInfo.assets[0])
	require.Equal(t, assetInfo2, innerAppInfo.assets[1])
	/*
		// 4. appSwapOuterCall -> appSwapOuterCreate
		numTxns = 3
		txnCounter += uint64(numTxns)
		actual, sgnTxns, objID, err = g.generateAppCallInternal(appSwapOuterCall, round, intra, &hint)
		require.Equal(t, appSwapOuterCreate, actual)
		require.Len(t, sgnTxns, 1)
		swapInnerCreate = sgnTxns[0].Txn
		require.Equal(t, protocol.ApplicationCallTx, swapInnerCreate.Type)
		require.Equal(t, transactions.NoOpOC, swapInnerCreate.ApplicationCallTxnFields.OnCompletion)
		require.Equal(t, assembled.swapInnerApproval, swapInnerCreate.ApplicationCallTxnFields.ApprovalProgram)
		require.Equal(t, assembled.swapInnerClear, swapInnerCreate.ApplicationCallTxnFields.ClearStateProgram)
		require.Equal(t, txnCounter, objID)
		// assert also that the opted in assets for objID are assetID1 and assetID2
		require.NoError(t, err)

		mockAdvance(numTxns)
		// sanity check
		require.Equal(t, txnCounter, g.txnCounter)

		// 5. appSwapOuterCall -> appSwapOuterOptin
		// 6. appSwapOuterCall -> appSwapOuterCall		// Finally after 6 rounds of trying!
	*/
}

func TestWriteRoundZero(t *testing.T) {
	partitiontest.PartitionTest(t)
	var testcases = []struct {
		name    string
		dbround uint64
		round   uint64
		genesis bookkeeping.Genesis
	}{
		{
			name:    "empty database",
			dbround: 0,
			round:   0,
			genesis: bookkeeping.Genesis{},
		},
		{
			name:    "preloaded database",
			dbround: 1,
			round:   1,
			genesis: bookkeeping.Genesis{Network: "TestWriteRoundZero"},
		},
	}
	for _, tc := range testcases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			g := makePrivateGenerator(t, tc.dbround, tc.genesis)
			var data []byte
			writer := bytes.NewBuffer(data)
			g.WriteBlock(writer, tc.round)
			var block rpcs.EncodedBlockCert
			protocol.Decode(data, &block)
			require.Len(t, block.Block.Payset, 0)
			g.ledger.Close()
		})
	}

}

func TestWriteRound(t *testing.T) {
	partitiontest.PartitionTest(t)
	g := makePrivateGenerator(t, 0, bookkeeping.Genesis{})

	prepBuffer := func() (*bytes.Buffer, rpcs.EncodedBlockCert) {
		return bytes.NewBuffer([]byte{}), rpcs.EncodedBlockCert{}
	}

	// Initial conditions of g from makePrivateGenerator:
	require.Equal(t, uint64(0), g.round)

	// Round 0:
	blockBuff, block0_1 := prepBuffer()
	err := g.WriteBlock(blockBuff, 0)
	require.NoError(t, err)

	require.Equal(t, uint64(1), g.round)
	protocol.Decode(blockBuff.Bytes(), &block0_1)
	require.Equal(t, "blockgen-test", block0_1.Block.BlockHeader.GenesisID)
	require.Equal(t, basics.Round(0), block0_1.Block.BlockHeader.Round)
	require.NotNil(t, g.ledger)
	require.Equal(t, basics.Round(0), g.ledger.Latest())

	// WriteBlocks only advances the _internal_ round
	// the first time called for a particular _given_ round
	blockBuff, block0_2 := prepBuffer()
	err = g.WriteBlock(blockBuff, 0)
	require.NoError(t, err)
	require.Equal(t, uint64(1), g.round)
	protocol.Decode(blockBuff.Bytes(), &block0_2)
	require.Equal(t, block0_1, block0_2)
	require.NotNil(t, g.ledger)
	require.Equal(t, basics.Round(0), g.ledger.Latest())

	blockBuff, block0_3 := prepBuffer()
	err = g.WriteBlock(blockBuff, 0)
	require.NoError(t, err)
	require.Equal(t, uint64(1), g.round)
	protocol.Decode(blockBuff.Bytes(), &block0_3)
	require.Equal(t, block0_1, block0_3)
	require.NotNil(t, g.ledger)
	require.Equal(t, basics.Round(0), g.ledger.Latest())

	// Round 1:
	blockBuff, block1_1 := prepBuffer()
	err = g.WriteBlock(blockBuff, 1)
	require.NoError(t, err)
	require.Equal(t, uint64(2), g.round)
	protocol.Decode(blockBuff.Bytes(), &block1_1)
	require.Equal(t, "blockgen-test", block1_1.Block.BlockHeader.GenesisID)
	require.Equal(t, basics.Round(1), block1_1.Block.BlockHeader.Round)
	require.Len(t, block1_1.Block.Payset, int(g.config.TxnPerBlock))
	require.NotNil(t, g.ledger)
	require.Equal(t, basics.Round(1), g.ledger.Latest())
	_, err = g.ledger.GetStateDeltaForRound(1)
	require.NoError(t, err)

	blockBuff, block1_2 := prepBuffer()
	err = g.WriteBlock(blockBuff, 1)
	require.NoError(t, err)
	require.Equal(t, uint64(2), g.round)
	protocol.Decode(blockBuff.Bytes(), &block1_2)
	require.Equal(t, block1_1, block1_2)
	require.NotNil(t, g.ledger)
	require.Equal(t, basics.Round(1), g.ledger.Latest())
	_, err = g.ledger.GetStateDeltaForRound(1)
	require.NoError(t, err)

	// request a block that is several rounds ahead of the current round
	err = g.WriteBlock(blockBuff, 10)
	require.NotNil(t, err)
	require.Equal(t, err.Error(), "generator only supports sequential block access. Expected 1 or 2 but received request for 10")
}

func TestWriteRoundWithPreloadedDB(t *testing.T) {
	partitiontest.PartitionTest(t)
	var testcases = []struct {
		name    string
		dbround uint64
		round   uint64
		genesis bookkeeping.Genesis
		err     error
	}{
		{
			name:    "preloaded database starting at round 1",
			dbround: 1,
			round:   1,
			genesis: bookkeeping.Genesis{Network: "generator-test1"},
		},
		{
			name:    "invalid request",
			dbround: 10,
			round:   1,
			genesis: bookkeeping.Genesis{Network: "generator-test2"},
			err:     fmt.Errorf("cannot generate block for round 1, already in database"),
		},
		{
			name:    "invalid request 2",
			dbround: 1,
			round:   10,
			genesis: bookkeeping.Genesis{Network: "generator-test3"},
			err:     fmt.Errorf("generator only supports sequential block access. Expected 1 or 2 but received request for 10"),
		},
		{
			name:    "preloaded database starting at 10",
			dbround: 10,
			round:   11,
			genesis: bookkeeping.Genesis{Network: "generator-test4"},
		},
		{
			name:    "preloaded database request round 20",
			dbround: 10,
			round:   20,
			genesis: bookkeeping.Genesis{Network: "generator-test5"},
		},
	}
	for _, tc := range testcases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			// No t.Parallel() here, to avoid contention in the ledger
			g := makePrivateGenerator(t, tc.dbround, tc.genesis)

			defer g.ledger.Close()
			var data []byte
			writer := bytes.NewBuffer(data)
			err := g.WriteBlock(writer, tc.dbround)
			require.Nil(t, err)
			// invalid block request
			if tc.round != tc.dbround && tc.err != nil {
				err = g.WriteBlock(writer, tc.round)
				require.NotNil(t, err)
				require.Equal(t, tc.err.Error(), err.Error())
				return
			}
			// write the rest of the blocks
			for i := tc.dbround + 1; i <= tc.round; i++ {
				err = g.WriteBlock(writer, i)
				require.Nil(t, err)
			}
			var block rpcs.EncodedBlockCert
			protocol.Decode(data, &block)
			require.Len(t, block.Block.Payset, int(g.config.TxnPerBlock))
			require.NotNil(t, g.ledger)
			require.Equal(t, basics.Round(tc.round-tc.dbround), g.ledger.Latest())
			if tc.round > tc.dbround {
				_, err = g.ledger.GetStateDeltaForRound(basics.Round(tc.round - tc.dbround))
				require.NoError(t, err)
			}
		})
	}
}

func TestHandlers(t *testing.T) {
	partitiontest.PartitionTest(t)
	g := makePrivateGenerator(t, 0, bookkeeping.Genesis{})
	handler := getBlockHandler(g)
	var testcases = []struct {
		name string
		url  string
		err  string
	}{
		{
			name: "no block",
			url:  "/v2/blocks/?nothing",
			err:  "invalid request path, /",
		},
		{
			name: "blocks: round must be numeric",
			url:  "/v2/blocks/round",
			err:  `strconv.ParseUint: parsing "round": invalid syntax`,
		},
		{
			name: "deltas: round must be numeric",
			url:  "/v2/deltas/round",
			err:  `strconv.ParseUint: parsing "round": invalid syntax`,
		},
	}

	for _, testcase := range testcases {
		testcase := testcase
		t.Run(testcase.name, func(t *testing.T) {
			t.Parallel()
			req := httptest.NewRequest("GET", testcase.url, nil)
			w := httptest.NewRecorder()
			handler(w, req)
			require.Equal(t, http.StatusBadRequest, w.Code)
			require.Contains(t, w.Body.String(), testcase.err)
		})
	}
}

func TestRecordData(t *testing.T) {
	gen := makePrivateGenerator(t, 0, bookkeeping.Genesis{})

	id := TxTypeID("test")
	data, ok := gen.reportData[id]
	require.False(t, ok)

	gen.recordData(id, time.Now())
	data, ok = gen.reportData[id]
	require.True(t, ok)
	require.Equal(t, uint64(1), data.GenerationCount)

	gen.recordData(id, time.Now())
	data, ok = gen.reportData[id]
	require.True(t, ok)
	require.Equal(t, uint64(2), data.GenerationCount)

}

func TestRecordOccurrences(t *testing.T) {
	gen := makePrivateGenerator(t, 0, bookkeeping.Genesis{})

	id := TxTypeID("test")
	data, ok := gen.reportData[id]
	require.False(t, ok)

	gen.recordOccurrences(id, 100, time.Now())
	data, ok = gen.reportData[id]
	require.True(t, ok)
	require.Equal(t, uint64(100), data.GenerationCount)

	gen.recordOccurrences(id, 200, time.Now())
	data, ok = gen.reportData[id]
	require.True(t, ok)
	require.Equal(t, uint64(300), data.GenerationCount)
}

func TestRecordAppConsequences(t *testing.T) {
	g := makePrivateGenerator(t, 0, bookkeeping.Genesis{})

	txTypeId := TxTypeID("test")
	txCount, err := g.countAndRecordEffects(txTypeId, time.Now())
	require.Error(t, err, "no effects for TxTypeId test")

	// recordIncludingEffects always records the root txTypeId
	require.Equal(t, uint64(1), txCount)
	data, ok := g.reportData[txTypeId]
	require.True(t, ok)
	require.Equal(t, uint64(1), data.GenerationCount)
	require.Len(t, g.reportData, 1)

	txTypeId = appBoxesOptin
	txCount, err = g.countAndRecordEffects(txTypeId, time.Now())
	require.NoError(t, err)
	require.Equal(t, uint64(4), txCount)

	require.Len(t, g.reportData, 4)

	data, ok = g.reportData[txTypeId]
	require.True(t, ok)
	require.Equal(t, uint64(1), data.GenerationCount)

	data, ok = g.reportData[effectSiblingPay]
	require.True(t, ok)
	require.Equal(t, uint64(1), data.GenerationCount)

	data, ok = g.reportData[effectInnerPay]
	require.True(t, ok)
	require.Equal(t, uint64(2), data.GenerationCount)
}
