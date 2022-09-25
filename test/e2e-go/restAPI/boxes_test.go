package restapi

import (
	"encoding/binary"
	"sort"
	"testing"
	"time"

	"github.com/algorand/go-algorand/crypto"
	"github.com/algorand/go-algorand/daemon"
	algodClient "github.com/algorand/go-algorand/daemon/algod/api/client"
	"github.com/algorand/go-algorand/daemon/algod/api/server/v2/generated"
	kmdclient "github.com/algorand/go-algorand/daemon/kmd/client"
	"github.com/algorand/go-algorand/data/basics"
	"github.com/algorand/go-algorand/data/transactions"
	"github.com/algorand/go-algorand/data/transactions/logic"
	"github.com/algorand/go-algorand/libgoal"
	"github.com/stretchr/testify/require"
)

func traceBoxes(t *testing.T, a *require.Assertions, _ algodClient.RestClient, gc libgoal.Client) []daemon.Trace{
	liveTraces := []daemon.Trace{}
	
	// trace := &daemon.Trace{Name: "nil program works, but results in invalid version text"}
	gc.WaitForRound(1)

	gc.SetAPIVersionAffinity(algodClient.APIVersionV2, kmdclient.APIVersionV1)

	wh, err := gc.GetUnencryptedWalletHandle()
	a.NoError(err)
	addresses, err := gc.ListAddresses(wh)
	a.NoError(err)
	_, someAddress := getMaxBalAddr(t, gc, addresses)
	if someAddress == "" {
		a.Fail("no addr with funds")
	}
	a.NoError(err)

	prog := `#pragma version 8
    txn ApplicationID
    bz end
    txn ApplicationArgs 0   // [arg[0]] // fails if no args && app already exists
    byte "create"           // [arg[0], "create"] // create box named arg[1]
    ==                      // [arg[0]=?="create"]
    bz del                  // "create" ? continue : goto del
    int 5                   // [5]
    txn ApplicationArgs 1   // [5, arg[1]]
    swap
    box_create              // [] // boxes: arg[1] -> [5]byte
    assert
    b end
del:                        // delete box arg[1]
    txn ApplicationArgs 0   // [arg[0]]
    byte "delete"           // [arg[0], "delete"]
    ==                      // [arg[0]=?="delete"]
	bz set                  // "delete" ? continue : goto set
    txn ApplicationArgs 1   // [arg[1]]
    box_del                 // del boxes[arg[1]]
    assert
    b end
set:						// put arg[1] at start of box arg[0] ... so actually a _partial_ "set"
    txn ApplicationArgs 0   // [arg[0]]
    byte "set"              // [arg[0], "set"]
    ==                      // [arg[0]=?="set"]
    bz bad                  // "delete" ? continue : goto bad
    txn ApplicationArgs 1   // [arg[1]]
    int 0                   // [arg[1], 0]
    txn ApplicationArgs 2   // [arg[1], 0, arg[2]]
    box_replace             // [] // boxes: arg[1] -> replace(boxes[arg[1]], 0, arg[2])
    b end
bad:
    err
end:
    int 1
`
	ops, err := logic.AssembleString(prog)
	approval := ops.Program
	ops, err = logic.AssembleString("#pragma version 8\nint 1")
	clearState := ops.Program

	gl := basics.StateSchema{}
	lc := basics.StateSchema{}

	// create app
	appCreateTxn, err := gc.MakeUnsignedApplicationCallTx(
		0, nil, nil, nil,
		nil, nil, transactions.NoOpOC,
		approval, clearState, gl, lc, 0,
	)
	a.NoError(err)
	appCreateTxn, err = gc.FillUnsignedTxTemplate(someAddress, 0, 0, 0, appCreateTxn)
	a.NoError(err)
	appCreateTxID, err := gc.SignAndBroadcastTransaction(wh, nil, appCreateTxn)
	a.NoError(err)
	_, err = waitForTransaction(t, gc, someAddress, appCreateTxID, 30*time.Second)
	a.NoError(err)

	// get app ID
	submittedAppCreateTxn, err := gc.PendingTransactionInformationV2(appCreateTxID)
	a.NoError(err)
	a.NotNil(submittedAppCreateTxn.ApplicationIndex)
	createdAppID := basics.AppIndex(*submittedAppCreateTxn.ApplicationIndex)
	a.Greater(uint64(createdAppID), uint64(0))

	// fund app account
	appFundTxn, err := gc.SendPaymentFromWallet(
		wh, nil, someAddress, createdAppID.Address().String(),
		0, 10_000_000, nil, "", 0, 0,
	)
	a.NoError(err)
	appFundTxID := appFundTxn.ID()
	_, err = waitForTransaction(t, gc, someAddress, appFundTxID.String(), 30*time.Second)
	a.NoError(err)

	createdBoxName := map[string]bool{}
	var createdBoxCount uint64 = 0

	// define operate box helper
	operateBoxAndSendTxn := func(operation string, boxNames []string, boxValues []string) {
		txns := make([]transactions.Transaction, len(boxNames))
		txIDs := make(map[string]string, len(boxNames))

		for i := 0; i < len(boxNames); i++ {
			appArgs := [][]byte{
				[]byte(operation),
				[]byte(boxNames[i]),
				[]byte(boxValues[i]),
			}
			boxRef := transactions.BoxRef{
				Name:  []byte(boxNames[i]),
				Index: 0,
			}

			txns[i], err = gc.MakeUnsignedAppNoOpTx(
				uint64(createdAppID), appArgs,
				nil, nil, nil,
				[]transactions.BoxRef{boxRef},
			)
			a.NoError(err)
			txns[i], err = gc.FillUnsignedTxTemplate(someAddress, 0, 0, 0, txns[i])
			a.NoError(err)
			txIDs[txns[i].ID().String()] = someAddress
		}

		var gid crypto.Digest
		gid, err = gc.GroupID(txns)
		a.NoError(err)

		stxns := make([]transactions.SignedTxn, len(boxNames))
		for i := 0; i < len(boxNames); i++ {
			txns[i].Group = gid
			wh, err = gc.GetUnencryptedWalletHandle()
			a.NoError(err)
			stxns[i], err = gc.SignTransactionWithWallet(wh, nil, txns[i])
			a.NoError(err)
		}

		err = gc.BroadcastTransactionGroup(stxns)
		a.NoError(err)

		_, err = waitForTransaction(t, gc, someAddress, txns[0].ID().String(), 30*time.Second)
		a.NoError(err)
	}

	operateAndMatchResCounter := 0
	// helper function, take operation and a slice of box names
	// then submit transaction group containing all operations on box names
	// Then we check these boxes are appropriately created/deleted
	operateAndMatchRes := func(operation string, boxNames []string) {
		operateAndMatchResCounter++
		boxValues := make([]string, len(boxNames))
		if operation == "create" {
			for i, box := range boxNames {
				keyValid, ok := createdBoxName[box]
				a.False(ok && keyValid)
				boxValues[i] = ""
			}
		} else if operation == "delete" {
			for i, box := range boxNames {
				keyValid, ok := createdBoxName[box]
				a.True(keyValid == ok)
				boxValues[i] = ""
			}
		} else {
			a.Failf("Unknown operation %s", operation)
		}

		operateBoxAndSendTxn(operation, boxNames, boxValues)

		if operation == "create" {
			for _, box := range boxNames {
				createdBoxName[box] = true
			}
			createdBoxCount += uint64(len(boxNames))
		} else if operation == "delete" {
			for _, box := range boxNames {
				createdBoxName[box] = false
			}
			createdBoxCount -= uint64(len(boxNames))
		}

		var resp generated.BoxesResponse
		gc.StartTrace("boxes request for %d (%d)", createdAppID, operateAndMatchResCounter)
		resp, err = gc.ApplicationBoxes(uint64(createdAppID), 0)
		liveTraces = append(liveTraces, *gc.Trace())
		a.NoError(err)
		

		expectedCreatedBoxes := make([]string, 0, createdBoxCount)
		for name, isCreate := range createdBoxName {
			if isCreate {
				expectedCreatedBoxes = append(expectedCreatedBoxes, name)
			}
		}
		sort.Strings(expectedCreatedBoxes)

		actualBoxes := make([]string, len(resp.Boxes))
		for i, box := range resp.Boxes {
			actualBoxes[i] = string(box.Name)
		}
		sort.Strings(actualBoxes)

		a.Equal(expectedCreatedBoxes, actualBoxes)
	}

	testingBoxNames := []string{
		` `,
		`     	`,
		` ? = % ;`,
		`; DROP *;`,
		`OR 1 = 1;`,
		`"      ;  SELECT * FROM kvstore; DROP acctrounds; `,
		`背负青天而莫之夭阏者，而后乃今将图南。`,
		`於浩歌狂熱之際中寒﹔於天上看見深淵。`,
		`於一切眼中看見無所有﹔於無所希望中得救。`,
		`有一遊魂，化為長蛇，口有毒牙。`,
		`不以嚙人，自嚙其身，終以殞顛。`,
		`那些智力超常的人啊`,
		`认为已经，熟悉了云和闪电的脾气`,
		`就不再迷惑，就不必了解自己，世界和他人`,
		`每天只管，被微风吹拂，与猛虎谈情`,
		`他们从来，不需要楼梯，只有窗口`,
		`把一切交付于梦境，和优美的浪潮`,
		`在这颗行星所有的酒馆，青春自由似乎理所应得`,
		`面向涣散的未来，只唱情歌，看不到坦克`,
		`在科学和啤酒都不能安抚的夜晚`,
		`他们丢失了四季，惶惑之行开始`,
		`这颗行星所有的酒馆，无法听到远方的呼喊`,
		`野心勃勃的灯火，瞬间吞没黑暗的脸庞`,
		`b64:APj/AA==`,
		`str:123.3/aa\\0`,
		string([]byte{0, 255, 254, 254}),
		string([]byte{0, 0xF8, 0xF9, 0xFA, 0xFB, 0xFC, 0xFD, 0xFE, 0xFF}),
		`; SELECT key from kvstore WHERE key LIKE %;`,
		`?&%!=`,
		"SELECT * FROM kvstore " + string([]byte{0, 0}) + " WHERE key LIKE %; ",
		string([]byte{'%', 'a', 'b', 'c', 0, 0, '%', 'a', '!'}),
		`
`,
		`™£´´∂ƒ∂ƒßƒ©∑®ƒß∂†¬∆`,
		`∑´´˙©˚¬∆ßåƒ√¬`,
	}

	gc.StartTrace("request for boxes")
	resp, err := gc.ApplicationBoxes(uint64(createdAppID), 0)
	liveTraces = append(liveTraces, *gc.Trace())
	a.NoError(err)
	a.Empty(resp.Boxes)

	for i := 0; i < len(testingBoxNames); i += 16 {
		var strSliceTest []string
		// grouping box names to operate, and create such boxes
		if i+16 >= len(testingBoxNames) {
			strSliceTest = testingBoxNames[i:]
		} else {
			strSliceTest = testingBoxNames[i : i+16]
		}
		operateAndMatchRes("create", strSliceTest)
	}

	maxBoxNumToGet := uint64(10)
	gc.StartTrace("lots o boxes (%d)", maxBoxNumToGet)
	resp, err = gc.ApplicationBoxes(uint64(createdAppID), maxBoxNumToGet)
	liveTraces = append(liveTraces, *gc.Trace())

	a.NoError(err)
	a.Len(resp.Boxes, int(maxBoxNumToGet))

	for i := 0; i < len(testingBoxNames); i += 16 {
		var strSliceTest []string
		// grouping box names to operate, and delete such boxes
		if i+16 >= len(testingBoxNames) {
			strSliceTest = testingBoxNames[i:]
		} else {
			strSliceTest = testingBoxNames[i : i+16]
		}
		operateAndMatchRes("delete", strSliceTest)
	}

	gc.StartTrace("more empty boxes")
	resp, err = gc.ApplicationBoxes(uint64(createdAppID), 0)
	liveTraces = append(liveTraces, *gc.Trace())
	a.NoError(err)
	a.Empty(resp.Boxes)

	// Get Box value from box name
	encodeInt := func(n uint64) []byte {
		ibytes := make([]byte, 8)
		binary.BigEndian.PutUint64(ibytes, n)
		return ibytes
	}

	boxTests := []struct {
		name        []byte
		encodedName string
		value       []byte
	}{
		{[]byte("foo"), "str:foo", []byte("bar12")},
		{encodeInt(12321), "int:12321", []byte{0, 1, 254, 3, 2}},
		{[]byte{0, 248, 255, 32}, "b64:APj/IA==", []byte("lux56")},
	}
	for _, boxTest := range boxTests {
		// Box values are 5 bytes, as defined by the test TEAL program.
		operateBoxAndSendTxn("create", []string{string(boxTest.name)}, []string{""})
		operateBoxAndSendTxn("set", []string{string(boxTest.name)}, []string{string(boxTest.value)})

		gc.StartTrace("looking for box %s", boxTest.encodedName)
		boxResponse, err := gc.GetApplicationBoxByName(uint64(createdAppID), boxTest.encodedName)
		liveTraces = append(liveTraces, *gc.Trace())
		a.NoError(err)
		a.Equal(boxTest.name, boxResponse.Name)
		a.Equal(boxTest.value, boxResponse.Value)
	}

	return liveTraces
}

/*
go test -v github.com/algorand/go-algorand/test/e2e-go/restapi -run="TestBoxes"
*/


func TestBoxes(t *testing.T) {
	tracingTest(t, traceBoxes, false /* developerAPI */, "boxes.json")
}