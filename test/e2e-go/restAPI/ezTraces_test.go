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

// type Client interface {algodClient.RestClient | kmdClient.KMDClient | libgoal.Client}

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

func assertNoRegressions(a *require.Assertions, savedTraces []daemon.Trace, liveTraces []daemon.Trace) {
	a.Len(liveTraces, len(savedTraces))
	for i, savedTrace := range savedTraces {
		liveTrace := liveTraces[i]
		a.Equal(savedTrace.Daemon, liveTrace.Daemon)
		a.Equal(savedTrace.Name, liveTrace.Name)
		a.Equal(savedTrace.Path, liveTrace.Path)
		a.Equal(savedTrace.Resource, liveTrace.Resource)
		a.Equal(savedTrace.Method, liveTrace.Method)
		a.Equal(savedTrace.BytesB64, liveTrace.BytesB64)
		a.Equal(savedTrace.Params, liveTrace.Params)
		a.Equal(savedTrace.EncodeJSON, liveTrace.EncodeJSON)
		a.Equal(savedTrace.DecodeJSON, liveTrace.DecodeJSON)
		a.Equal(savedTrace.StatusCode, liveTrace.StatusCode)
		a.Equal(savedTrace.ResponseErr, liveTrace.ResponseErr)
		a.Equal(savedTrace.Response, liveTrace.Response)
		a.Equal(savedTrace.ResponseB64, liveTrace.ResponseB64)
		a.Equal(savedTrace.ParsedResponseType, liveTrace.ParsedResponseType)

		if savedTrace.ParsedResponse == nil {
			a.Nil(liveTrace.ParsedResponse)
		} else {
			var recovered any
			switch savedTrace.ParsedResponseType {
			case "*generated.DisassembleResponse":
				recovered = recoverType[*generated.DisassembleResponse](a, savedTrace.ParsedResponse)
			default:
				a.Fail("unknown savedTrace.ParsedResponseType %s", savedTrace.ParsedResponseType)
			}
			a.Equal(recovered, liveTrace.ParsedResponse)
		}
	}
}

// The trace results are saved in ./{tracesDirectory}/_{ezTracesFile}
// and are compared against ./{tracesDirectory}/{ezTracesFile}
func tracingTest(t *testing.T, tracer tracerTest, developerAPI bool, tracesFile string) {
	// Setup an EZ Trace Test:
	a, algodClient, goalClient, shutDown := SetupTraces(t, developerAPI)
	defer shutDown()

	liveTraces := tracer(a, algodClient, goalClient)

	// Save the traces to a non source controlled file:
	writeTraces(a, liveTraces, "_"+tracesFile)

	// Read the source controlled traces file:
	savedTraces := readTraces(a, tracesFile)

	// Compare liveTraces vs. saved Traces
	assertNoRegressions(a, savedTraces, liveTraces)
}