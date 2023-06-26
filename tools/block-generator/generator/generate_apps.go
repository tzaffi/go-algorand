package generator

import (
	"fmt"
	"math/rand"
	"time"

	txn "github.com/algorand/go-algorand/data/transactions"
)

// ---- generator app state ----

func (g *generator) resetPendingApps() {
	g.pendingAppSlice = map[appKind][]*appData{
		appKindBoxes: make([]*appData, 0),
		appKindSwap:  make([]*appData, 0),
	}
	g.pendingAppMap = map[appKind]map[uint64]*appData{
		appKindBoxes: make(map[uint64]*appData),
		appKindSwap:  make(map[uint64]*appData),
	}
}

// ---- effects and consequences ----

// effects is a map that contains the hard-coded non-trivial
// consequents of a transaction type.
// The "sibling" transactions are added to an atomic transaction group
// in a "makeXyzTransaction" function defined in make_transactions.go.
// The "inner" transactions are created inside the TEAL programs. See:
// * teal/poap_boxes.teal
// * teal/swap_amm.teal
//
// appBoxesCreate: 1 sibling payment tx
// appBoxesOptin: 1 sibling payment tx, 2 inner tx
var effects = map[TxTypeID][]TxEffect{
	appBoxesCreate: {
		{effectPaymentTxSibling, 1},
	},
	appBoxesOptin: {
		{effectPaymentTxSibling, 1},
		{effectInnerTx, 2},
	},
}

func countEffects(actual TxTypeID) uint64 {
	cnt := uint64(0)
	if effectsFromActual, ok := effects[actual]; ok {
		for _, effect := range effectsFromActual {
			cnt += effect.count
		}
	}
	return cnt
}

func CumulativeEffects(report Report) EffectsReport {
	effsReport := make(EffectsReport)
	for txType, data := range report {
		rootCount := data.GenerationCount
		effsReport[string(txType)] += rootCount
		for _, effect := range effects[txType] {
			effsReport[effect.effect] += effect.count * rootCount
		}
	}
	return effsReport
}

// ---- 3. App Transactions ----

func (g *generator) generateAppTxn(round uint64, intra uint64) ([]txn.SignedTxn, uint64 /* nextIntra */, uint64 /* appID */, error) {
	start := time.Now()
	selection, err := weightedSelection(g.appTxWeights, getAppTxOptions(), appSwapCall)
	if err != nil {
		return nil, intra, 0, err
	}

	actual, signedTxns, appID, err := g.generateAppCallInternal(selection.(TxTypeID), round, intra, nil)
	if err != nil {
		return nil, intra, appID, fmt.Errorf("unexpected error received from generateAppCallInternal(): %w", err)
	}

	intra += 1 + countEffects(actual) // +1 for actual

	g.recordData(actual, start)
	return signedTxns, intra, appID, nil
}

// generateAppCallInternal is the main workhorse for generating app transactions.
// Senders are always genesis accounts to avoid running out of funds.
func (g *generator) generateAppCallInternal(txType TxTypeID, round, intra uint64, hintApp *appData) (TxTypeID, []txn.SignedTxn, uint64 /* appID */, error) {
	var senderIndex uint64
	if hintApp != nil {
		senderIndex = hintApp.sender
	} else {
		senderIndex = rand.Uint64() % g.config.NumGenesisAccounts
	}
	senderAcct := indexToAccount(senderIndex)

	actual, kind, appCallType, appID, err := g.getActualAppCall(txType, senderIndex)
	if err != nil {
		return "", nil, appID, err
	}
	if hintApp != nil && hintApp.appID != 0 {
		// can only override the appID when non-zero in hintApp
		appID = hintApp.appID
	}
	// WLOG: the matched cases below are now well-defined thanks to getActualAppCall()

	var signedTxns []txn.SignedTxn
	switch appCallType {
	case appTxTypeCreate:
		appID = g.txnCounter + intra + 1
		signedTxns = g.makeAppCreateTxn(kind, senderAcct, round, intra, appID)
		reSignTxns(signedTxns)

		for k := range g.appMap {
			if g.appMap[k][appID] != nil {
				return "", nil, appID, fmt.Errorf("should never happen! app %d already exists for kind %s", appID, k)
			}
			if g.pendingAppMap[k][appID] != nil {
				return "", nil, appID, fmt.Errorf("should never happen! app %d already pending for kind %s", appID, k)
			}
		}

		ad := &appData{
			appID:  appID,
			sender: senderIndex,
			kind:   kind,
			optins: map[uint64]bool{},
		}

		g.pendingAppSlice[kind] = append(g.pendingAppSlice[kind], ad)
		g.pendingAppMap[kind][appID] = ad

	case appTxTypeOptin:
		signedTxns = g.makeAppOptinTxn(senderAcct, round, intra, kind, appID)
		reSignTxns(signedTxns)
		if g.pendingAppMap[kind][appID] == nil {
			ad := &appData{
				appID:  appID,
				sender: senderIndex,
				kind:   kind,
				optins: map[uint64]bool{},
			}
			g.pendingAppMap[kind][appID] = ad
			g.pendingAppSlice[kind] = append(g.pendingAppSlice[kind], ad)
		}
		g.pendingAppMap[kind][appID].optins[senderIndex] = true

	case appTxTypeCall:
		signedTxns = []txn.SignedTxn{
			signTxn(g.makeAppCallTxn(senderAcct, round, intra, appID)),
		}

	default:
		return "", nil, appID, fmt.Errorf("unimplemented: invalid transaction type <%s> for app %d", appCallType, appID)
	}

	return actual, signedTxns, appID, nil
}

func (g *generator) getAppData(existing bool, kind appKind, senderIndex, appID uint64) (*appData, bool /* appInMap */, bool /* senderOptedin */) {
	var appMapOrPendingAppMap map[appKind]map[uint64]*appData
	if existing {
		appMapOrPendingAppMap = g.appMap
	} else {
		appMapOrPendingAppMap = g.pendingAppMap
	}

	ad, ok := appMapOrPendingAppMap[kind][appID]
	if !ok {
		return nil, false, false
	}
	if !ad.optins[senderIndex] {
		return ad, true, false
	}
	return ad, true, true
}

// getActualAppCall returns the actual transaction type, app kind, app transaction type and appID
// * it returns actual = txType if there aren't any problems (for example create always is kept)
// * it creates the app if the app of the given kind doesn't exist
// * it switches to noopoc instead of optin when already opted into existing apps
// * it switches to create instead of optin when only opted into pending apps
// * it switches to optin when noopoc if not opted in and follows the logic of the optins above
// * the appID is 0 for creates, and otherwise a random appID from the existing apps for the kind
func (g *generator) getActualAppCall(txType TxTypeID, senderIndex uint64) (TxTypeID, appKind, appTxType, uint64 /* appID */, error) {
	isApp, kind, appTxType, err := parseAppTxType(txType)
	if err != nil {
		return "", 0, 0, 0, err
	}
	if !isApp {
		return "", 0, 0, 0, fmt.Errorf("should be an app but not parsed that way: %v", txType)
	}

	// creates get a quick pass:
	if appTxType == appTxTypeCreate {
		return txType, kind, appTxTypeCreate, 0, nil
	}

	numAppsForKind := uint64(len(g.appSlice[kind]))
	if numAppsForKind == 0 {
		// can't do anything else with the app if it doesn't exist, so must create it first
		return getAppTxType(kind, appTxTypeCreate), kind, appTxTypeCreate, 0, nil
	}

	if appTxType == appTxTypeOptin {
		// pick a random app to optin:
		appID := g.appSlice[kind][rand.Uint64()%numAppsForKind].appID

		_, exists, optedIn := g.getAppData(true /* existing */, kind, senderIndex, appID)
		if !exists {
			return txType, kind, appTxType, appID, fmt.Errorf("should never happen! app %d of kind %s does not exist", appID, kind)
		}

		if optedIn {
			// already opted in, so call the app instead:
			return getAppTxType(kind, appTxTypeCall), kind, appTxTypeCall, appID, nil
		}

		_, _, optedInPending := g.getAppData(false /* pending */, kind, senderIndex, appID)
		if optedInPending {
			// about to get opted in, but can't optin twice or call yet, so create:
			return getAppTxType(kind, appTxTypeCreate), kind, appTxTypeCreate, appID, nil
		}
		// not opted in or pending, so optin:
		return txType, kind, appTxType, appID, nil
	}

	if appTxType != appTxTypeCall {
		return "", 0, 0, 0, fmt.Errorf("unimplemented transaction type for app %s from %s", appTxType, txType)
	}
	// WLOG appTxTypeCall:

	numAppsOptedin := uint64(len(g.accountAppOptins[kind][senderIndex]))
	if numAppsOptedin == 0 {
		// try again calling recursively but attempting to optin:
		return g.getActualAppCall(getAppTxType(kind, appTxTypeOptin), senderIndex)
	}
	// WLOG appTxTypeCall with available optins:

	appID := g.accountAppOptins[kind][senderIndex][rand.Uint64()%numAppsOptedin]
	return txType, kind, appTxType, appID, nil
}
