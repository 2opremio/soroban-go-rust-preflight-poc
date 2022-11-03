package main

// NOTE: There should be NO space between the comments and the `import "C"` line.
// The -ldl is sometimes necessary to fix linker errors about `dlsym`.

/*
#cgo LDFLAGS: ./lib/libpreflight.a -ldl
#include "./lib/preflight.h"
*/
import "C"
import (
	"fmt"
	"os"

	"github.com/hexops/valast"
	"github.com/stellar/go/xdr"
)

// test contract ID and source accounts
// we assume the increment contract in the soroban-examples repo to be used
// https://github.com/stellar/soroban-examples/blob/main/increment/src/lib.rs
var (
	contractID            = xdr.Hash{0xaa, 0xbb}
	sourceAccount         = xdr.MustAddress("GBTBXQEVDNVUEESCTPUT3CHJDVNG44EMPMBELH5F7H3YPHXPZXOTEWB4")
	sequenceNumber C.uint = 4456
)

func getContractCodeLedgerEntry() xdr.LedgerEntry {
	wasm, err := os.ReadFile("soroban_increment_contract.wasm")
	if err != nil {
		panic(err)
	}
	ic := xdr.ScStaticScsLedgerKeyContractCode
	obj := &xdr.ScObject{
		Type: xdr.ScObjectTypeScoContractCode,
		ContractCode: &xdr.ScContractCode{
			Type: xdr.ScContractCodeTypeSccontractCodeWasm,
			Wasm: &wasm,
		},
	}
	return xdr.LedgerEntry{
		LastModifiedLedgerSeq: xdr.Uint32(sequenceNumber) - 10,
		Data: xdr.LedgerEntryData{
			Type: xdr.LedgerEntryTypeContractData,
			ContractData: &xdr.ContractDataEntry{
				ContractId: contractID,
				Key: xdr.ScVal{
					Type: xdr.ScValTypeScvStatic,
					Ic:   &ic,
				},
				Val: xdr.ScVal{
					Type: xdr.ScValTypeScvObject,
					Obj:  &obj,
				},
			},
		},
	}
}

//export SnapshotSourceGet
func SnapshotSourceGet(ledger_key *C.char) *C.char {
	ledgerKeyB64 := C.GoString(ledger_key)
	var ledgerKey xdr.LedgerKey

	fmt.Println("Rust called SnapshotSourceGet()")
	if err := xdr.SafeUnmarshalBase64(ledgerKeyB64, &ledgerKey); err != nil {
		fmt.Printf("cannot unmarshal ledger key: %s", err)
		return nil
	}
	if ledgerKey.Type == xdr.LedgerEntryTypeContractData &&
		ledgerKey.ContractData.ContractId == contractID &&
		ledgerKey.ContractData.Key.Type == xdr.ScValTypeScvStatic &&
		*ledgerKey.ContractData.Key.Ic == xdr.ScStaticScsLedgerKeyContractCode {
		le := getContractCodeLedgerEntry()
		out, err := xdr.MarshalBase64(le)
		if err != nil {
			panic(err)
		}
		return C.CString(out)
	} else {
		fmt.Printf("Rust requested unknown ledger key: %s\n", valast.String(ledgerKey))
	}

	return nil
}

//export SnapshotSourceHas
func SnapshotSourceHas(ledger_key *C.char) C.int {
	return 0
}

func main() {
	hf := xdr.HostFunctionHostFnInvokeContract

	contractIdBytes := contractID[:]
	contractIdParameterObj := &xdr.ScObject{
		Type: xdr.ScObjectTypeScoBytes,
		Bin:  &contractIdBytes,
	}
	contractIdParameter := xdr.ScVal{
		Type: xdr.ScValTypeScvObject,
		Obj:  &contractIdParameterObj,
	}
	contractFnParameterSym := xdr.ScSymbol("increment")
	contractFnParameter := xdr.ScVal{
		Type: xdr.ScValTypeScvSymbol,
		Sym:  &contractFnParameterSym,
	}
	args := xdr.ScVec{
		contractIdParameter,
		contractFnParameter,
	}
	li := C.CLedgerInfo{
		protocol_version:   20,
		sequence_number:    sequenceNumber,
		timestamp:          1,
		network_passphrase: C.CString("test"),
		base_reserve:       1,
	}
	hfB64, err := xdr.MarshalBase64(hf)
	if err != nil {
		panic(err)
	}
	argsB64, err := xdr.MarshalBase64(args)
	if err != nil {
		panic(err)
	}
	sourceAccountB64, err := xdr.MarshalBase64(sourceAccount)
	if err != nil {
		panic(err)
	}
	res := C.preflight_host_function(C.CString(hfB64),
		C.CString(argsB64),
		C.CString(sourceAccountB64),
		li,
	)

	if res == nil {
		fmt.Println("preflight failed :(")
	}
	defer C.free_cstring(res)

	var preflight xdr.LedgerFootprint
	preflightB64 := C.GoString(res)
	if err := xdr.SafeUnmarshalBase64(preflightB64, &preflight); err != nil {
		panic(err)
	}

	fmt.Printf("Obtained preflight: %s\n", valast.String(preflight))
}
