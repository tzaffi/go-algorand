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

package restapi

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/algorand/go-algorand/config"
	"github.com/algorand/go-algorand/daemon"
	algodClient "github.com/algorand/go-algorand/daemon/algod/api/client"
	"github.com/algorand/go-algorand/daemon/algod/api/server/v2/generated"
	"github.com/algorand/go-algorand/libgoal"
	"github.com/algorand/go-algorand/test/framework/fixtures"
	"github.com/algorand/go-algorand/test/partitiontest"
	"github.com/stretchr/testify/require"
)

type tracerTest func(t *testing.T, a *require.Assertions, ac algodClient.RestClient, gc libgoal.Client) []daemon.Trace

// Set inspired by: https://dbuddy.medium.com/implementing-set-data-structure-in-go-using-generics-4a967f823bfb

type Set[T comparable] map[T]bool

func NewSet[T comparable]() Set[T] {
	return make(Set[T])
}
func AsSet[T comparable](elts ...T) (s Set[T]) {
	s = NewSet[T]()
	for _, elt := range elts {
		s[elt] = true
	}
	return
}
func AsSetOfStrings[T any](elts ...T)(s Set[string]){
	eltStrings := make([]string, len(elts))
	for _, elt := range elts {
		eltStrings = append(eltStrings, fmt.Sprintf("%+v", elt))
	}
	return AsSet(eltStrings...)
}

func Minus[T comparable](s1, s2 Set[T]) Set[T] {
	minus := []T{}
	for x := range s1 {
		if !s2[x] {
			minus = append(minus, x)
		}
	}
	return AsSet(minus...)
}

func Equal[T comparable](s1, s2 Set[T]) bool {
	if len(s1) != len(s2) {
		return false
	}
	for x := range s1 {
		if !s2[x] {
			return false
		}
	}
	return true
}

const tracesDirectory = "ezTraces"

func SetupTraces(t *testing.T, enableDeveloperAPI bool) (*require.Assertions, algodClient.RestClient, libgoal.Client, func()) {
	// TODO: there gotta be a better way!!!
	partitiontest.PartitionTest(t)

	a := require.New(fixtures.SynchronizedTest(t))

	var fixture fixtures.RestClientFixture
	fixture.SetupNoStart(t, filepath.Join("nettemplates", "OneNodeFuture.json"))

	// Get primary node
	// update the configuration file to enable the developer API
	primaryNode, err := fixture.GetNodeController("Primary")
	a.NoError(err)

	fixture.Start()

	if enableDeveloperAPI {
		primaryNode.FullStop()
		cfg, err := config.LoadConfigFromDisk(primaryNode.GetDataDir())
		a.NoError(err)
		cfg.EnableDeveloperAPI = true
		cfg.SaveToDisk(primaryNode.GetDataDir())
		fixture.Start()
	}

	restClient, err := fixture.NC.AlgodClient()
	require.NoError(t, err)

	return a, restClient, fixture.LibGoalClient, func() {
		fixtures.ShutdownSynchronizedTest(t)
		primaryNode.FullStop()
	}
}


func writeTraces(a *require.Assertions, traces []daemon.Trace, filename string) {
	writeBytes := marshal(a, traces)
	err := os.WriteFile(filepath.Join(tracesDirectory, filename), writeBytes, 0644)
	a.NoError(err)
}

func readTraces(a *require.Assertions, filename string) []daemon.Trace {
	fileBytes, err := os.ReadFile(filepath.Join(tracesDirectory, filename))
	a.NoError(err)

	return unmarshal[[]daemon.Trace](a, fileBytes)
}

func marshal(a *require.Assertions, r interface{}) []byte {
	writeBytes, err := json.MarshalIndent(r, "", "  ")
	a.NoError(err)
	return writeBytes
}

func unmarshal[R any](a *require.Assertions, readBytes []byte) (r R) {
	r = *new(R)
	err := json.Unmarshal(readBytes, &r)
	a.NoError(err)
	return
}

func recoverType[R any](a *require.Assertions, r interface{}) R {
	return unmarshal[R](a, marshal(a, r))
}

// var handledTypes map[string]reflect.Type{
// 	"*generated.DisassembleResponse": *generated.DisassembleResponse
// }

func assertNoRegressions(a *require.Assertions, savedTraces []daemon.Trace, liveTraces []daemon.Trace) {
	a.Len(liveTraces, len(savedTraces))
	for i, savedTrace := range savedTraces {
		liveTrace := liveTraces[i]
		msg := fmt.Sprintf("%d. %s", i, liveTrace.Name)
		e := func(x, y interface{}){
			a.Equal(x, y, msg)
		}

		e(savedTrace.Daemon, liveTrace.Daemon)
		e(savedTrace.Name, liveTrace.Name)
		e(savedTrace.Path, liveTrace.Path)
		e(savedTrace.Resource, liveTrace.Resource)
		e(savedTrace.Method, liveTrace.Method)
		e(savedTrace.BytesB64, liveTrace.BytesB64)
		e(savedTrace.Params, liveTrace.Params)
		e(savedTrace.EncodeJSON, liveTrace.EncodeJSON)
		e(savedTrace.DecodeJSON, liveTrace.DecodeJSON)
		e(savedTrace.StatusCode, liveTrace.StatusCode)
		e(savedTrace.ResponseErr, liveTrace.ResponseErr)
		e(savedTrace.ParsedResponseType, liveTrace.ParsedResponseType)

		equals, hasCustomEquals := customEquals[savedTrace.ParsedResponseType]

		if !hasCustomEquals {
			e(savedTrace.Response, liveTrace.Response)
			e(savedTrace.ResponseB64, liveTrace.ResponseB64)	
		} 
		
		if savedTrace.ParsedResponse == nil {
			a.Nil(liveTrace.ParsedResponse, msg)
		} else {
			recovered := recoverResponse(a, savedTrace)
			if !hasCustomEquals {
				e(recovered, liveTrace.ParsedResponse)
			} else {
				equals(a, recovered, liveTrace.ParsedResponse, msg)
			}
		}
	}
}

type equalityAsserter func(a *require.Assertions, x, y any, msgEtc ...interface{})


var customEquals = map[string]equalityAsserter{
	"*generated.BoxesResponse": func(a *require.Assertions, x, y any, msgEtc ...interface{}) {
		xbr, ok := x.(*generated.BoxesResponse)
		a.True(ok)
		ybr, ok := y.(*generated.BoxesResponse)
		a.True(ok)

		xSet := AsSetOfStrings(xbr.Boxes...)
		ySet := AsSetOfStrings(ybr.Boxes...)
		
		xyMinus := Minus(xSet, ySet)
		a.Empty(xyMinus, msgEtc...)

		yxMinus := Minus(ySet, xSet)
		a.Empty(yxMinus, msgEtc...)

		preamble := fmt.Sprintf("not equal as set of BoxDescriptor's: lengths %d v %d\ndetails:\n{%+v}\nvs\n{%+v}", len(xSet), len(ySet), xSet, ySet)
		msgEtc = append([]interface{}{preamble}, msgEtc...)
		a.True(Equal(xSet, ySet), msgEtc...)
	},
}

func recoverResponse(a *require.Assertions, savedTrace daemon.Trace) (recovered any) {
	parsed := savedTrace.ParsedResponse
	switch savedTrace.ParsedResponseType {
	case "*generated.DisassembleResponse":
		recovered = recoverType[*generated.DisassembleResponse](a, parsed)
	case "*generated.BoxesResponse":
		recovered = recoverType[*generated.BoxesResponse](a, parsed)
	case "*generated.BoxResponse":
		recovered = recoverType[*generated.BoxResponse](a, parsed)
	default:
		a.Fail("unknown savedTrace.ParsedResponseType %s", savedTrace.ParsedResponseType)
	}
	return
}

// The trace results are saved in ./{tracesDirectory}/_{ezTracesFile}
// and are compared against ./{tracesDirectory}/{ezTracesFile}
func tracingTest(t *testing.T, tracer tracerTest, developerAPI bool, tracesFile string) {
	// Setup an EZ Trace Test:
	a, algodClient, goalClient, shutDown := SetupTraces(t, developerAPI)
	defer shutDown()

	liveTraces := tracer(t, a, algodClient, goalClient)

	// Save the traces to a non source controlled file:
	writeTraces(a, liveTraces, "_"+tracesFile)

	// Read the source controlled traces file:
	savedTraces := readTraces(a, tracesFile)

	// Compare liveTraces vs. saved Traces
	assertNoRegressions(a, savedTraces, liveTraces)
}