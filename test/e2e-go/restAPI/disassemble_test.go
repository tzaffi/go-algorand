package restapi

import (
	"testing"

	"github.com/algorand/go-algorand/daemon"
	algodClient "github.com/algorand/go-algorand/daemon/algod/api/client"
	"github.com/algorand/go-algorand/data/transactions/logic"
	"github.com/algorand/go-algorand/libgoal"
	"github.com/stretchr/testify/require"
)

// Customized request tracer for `Disassemble()`:
func traceDisassemble(a *require.Assertions, ac algodClient.RestClient, _ libgoal.Client) []daemon.Trace {
	liveTraces := []daemon.Trace{}

	testProgram := []byte{}

	ac.StartTrace("nil program works, but results in invalid version text")
	resp, err := ac.Disassemble(testProgram)
	liveTraces = append(liveTraces, *ac.Trace())
	a.NoError(err)
	a.Equal("// invalid version\n", resp.Result)

	for ver := 1; ver <= logic.AssemblerMaxVersion; ver++ {
		goodProgram := `int 1`
		ops, _ := logic.AssembleStringWithVersion(goodProgram, uint64(ver))
		disassembledProgram, _ := logic.Disassemble(ops.Program)
		ac.StartTrace("Test a valid program across all assembler versions (v%d)", ver)
		resp, err = ac.Disassemble(ops.Program)
		liveTraces = append(liveTraces, *ac.Trace())

		a.NoError(err)
		a.Equal(disassembledProgram, resp.Result)
	}

	// NOTE: Intentionally _NOT_ testing without the developer API.
	// DeveloperAPI is assumed with intention to generate a fixture for the SDK's
	// which currently (September 2022) use a developer enabled node for testing

	badProgram := []byte{1, 99}
	ac.StartTrace("Test bad program")
	resp, err = ac.Disassemble(badProgram)
	liveTraces = append(liveTraces, *ac.Trace())
	a.ErrorContains(err, "invalid opcode 63 at pc=1")
	a.Equal("", resp.Result)

	return liveTraces
}


/*
go test -v github.com/algorand/go-algorand/test/e2e-go/restapi -run="TestDisassemble"
*/

func TestDisassemble(t *testing.T) {
	tracingTest(t, traceDisassemble, true /* developerAPI */, "disassemble.json")
}