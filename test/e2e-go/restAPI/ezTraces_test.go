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
	"github.com/algorand/go-algorand/daemon/algod/api/client"
	"github.com/algorand/go-algorand/daemon/algod/api/server/v2/generated"
	"github.com/algorand/go-algorand/data/transactions/logic"
	"github.com/algorand/go-algorand/test/framework/fixtures"
	"github.com/algorand/go-algorand/test/partitiontest"
	"github.com/stretchr/testify/require"
)

const tracesDirectory = "ezTraces"

func setupSynchronizedREST(t *testing.T, enableDeveloperAPI bool) (*require.Assertions, client.RestClient, func()) {
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

	return a, restClient, func() {
		fixtures.ShutdownSynchronizedTest(t)
		primaryNode.FullStop()
	}
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

func writeTraces(a *require.Assertions, traces []client.Trace, filename string) {
	writeBytes := marshal(a, traces)
	err := os.WriteFile(filepath.Join(tracesDirectory, filename), writeBytes, 0644)
	a.NoError(err)
}

func readTraces(a *require.Assertions, filename string) []client.Trace {
	fileBytes, err := os.ReadFile(filepath.Join(tracesDirectory, filename))
	a.NoError(err)

	return unmarshal[[]client.Trace](a, fileBytes)
}

func assertNoRegressions[R any](a *require.Assertions, savedTraces []client.Trace, liveTraces []client.Trace) {
	a.Len(liveTraces, len(savedTraces))
	for i, savedTrace := range savedTraces {
		liveTrace := liveTraces[i]
		a.Equal(savedTrace.Path, liveTrace.Path)
		a.Equal(savedTrace.Method, liveTrace.Method)
		a.Equal(savedTrace.BytesB64, liveTrace.BytesB64)
		a.Equal(savedTrace.Values, liveTrace.Values)
		a.Equal(savedTrace.EncodeJSON, liveTrace.EncodeJSON)
		a.Equal(savedTrace.DecodeJSON, liveTrace.DecodeJSON)
		a.Equal(savedTrace.StatusCode, liveTrace.StatusCode)
		a.Equal(savedTrace.ResponseErr, liveTrace.ResponseErr)
		a.Equal(savedTrace.ResponseB64, liveTrace.ResponseB64)
		a.Equal(savedTrace.ParsedResponseType, liveTrace.ParsedResponseType)

		if savedTrace.ParsedResponse == nil {
			a.Nil(liveTrace.ParsedResponse)
		} else {
			recoveredSavedResponse := recoverType[R](a, savedTrace.ParsedResponse)
			a.Equal(recoveredSavedResponse, liveTrace.ParsedResponse)
		}
	}
}

func traceDisassemble(a *require.Assertions, restClient client.RestClient) []client.Trace {
	liveTraces := []client.Trace{}

	testProgram := []byte{}

	// nil program works, but results in invalid version text.
	trace := new(client.Trace)
	resp, err := restClient.Disassemble(testProgram, trace)
	liveTraces = append(liveTraces, *trace)
	a.NoError(err)
	a.Equal("// invalid version\n", resp.Result)

	// Test a valid program across all assembler versions.
	for ver := 1; ver <= logic.AssemblerMaxVersion; ver++ {
		goodProgram := `int 1`
		ops, _ := logic.AssembleStringWithVersion(goodProgram, uint64(ver))
		disassembledProgram, _ := logic.Disassemble(ops.Program)
		trace := new(client.Trace)
		resp, err = restClient.Disassemble(ops.Program, trace)
		liveTraces = append(liveTraces, *trace)

		a.NoError(err)
		a.Equal(disassembledProgram, resp.Result)
	}

	// NOTE: Intentionally _NOT_ testing without the developer API.
	// DeveloperAPI is assumed with intention to generate a fixture for the SDK's
	// which currently (September 2022) use a developer enabled node for testing

	// Test bad program.
	badProgram := []byte{1, 99}
	trace = new(client.Trace)
	resp, err = restClient.Disassemble(badProgram, trace)
	liveTraces = append(liveTraces, *trace)
	a.ErrorContains(err, "invalid opcode 63 at pc=1")
	a.Equal("", resp.Result)

	return liveTraces
}

/*
go test -v github.com/algorand/go-algorand/test/e2e-go/restapi -run="TestDisassemble"
*/

func TestDisassemble(t *testing.T) {
	// Setup an EZ Trace Test:
	a, restClient, shutDown := setupSynchronizedREST(t, true /* enableDeveloperAPI */)
	defer shutDown()

	// The trace results are saved in ./{tracesDirectory}/_{ezTracesFile}
	// and are compared against ./{tracesDirectory}/{ezTracesFile}
	ezTracesFile := "disassemble.json"

	// Customized request tracer for `Disassemble()`:
	liveTraces := traceDisassemble(a, restClient)

	// Save the traces to a non source controlled file:
	writeTraces(a, liveTraces, "_"+ezTracesFile)

	// Read the source controlled traces file:
	savedTraces := readTraces(a, ezTracesFile)

	// Compare liveTraces vs. saved Traces
	assertNoRegressions[*generated.DisassembleResponse](a, savedTraces, liveTraces)
}
