package main

import (
	"bytes"
	"encoding/hex"
	"flag"
	"fmt"
	"io/ioutil"
	"math/big"
	"os"
	"strings"
	"time"

	"github.com/xunleichain/tc-wasm/cmd/tcvm/wasm"
	"github.com/xunleichain/tc-wasm/mock/log"
	"github.com/xunleichain/tc-wasm/mock/state"
	"github.com/xunleichain/tc-wasm/mock/types"
	"github.com/xunleichain/tc-wasm/vm"
	// "net/http"
	// _ "net/http/pprof"
)

var (
	helpParams = "-file path/to/testdata/tcvm.wasm -call path/to/testdata/tcvm.params"

	testAddr1 = types.BytesToAddress(types.Keccak256([]byte("addr-1 for call contract"))[:20])
	testAddr2 = types.BytesToAddress(types.Keccak256([]byte("addr-2 for contract"))[:20])

	testTime     = big.NewInt(1565078742)
	testBalance1 = big.NewInt(987650000999999999)
	testBalance2 = big.NewInt(987650000555555555)
	testGasPrice = big.NewInt(1999)
	testGasRate  = uint64(1000)

	wasmFileFlag  = flag.String("file", "", "file with wasm bytecode")
	callFuncFlag  = flag.String("call", "", "file with called function and data")
	contractGas   = flag.Uint64("gas", 52100, "contract msg gas")
	contractValue = flag.Uint64("value", 0, "contract msg value")
)

type MockChainContext struct {
	// GetHeader returns the hash corresponding to their hash.
}

func (m *MockChainContext) GetHeader(uint64) *types.Header { return &types.Header{} }

type MockAccountRef types.Address

// Address casts AccountRef to a Address
func (ar MockAccountRef) Address() types.Address { return (types.Address)(ar) }

func main() {
	flag.Parse()

	if len(*wasmFileFlag) == 0 {
		fmt.Printf("Usage:\n    %s %s\n\n", os.Args[0], helpParams)
		fmt.Printf("Use \"%s -h\" for more information\n", os.Args[0])
		return
	}

	byteCode, err := ioutil.ReadFile(*wasmFileFlag)
	if err != nil {
		fmt.Printf("ERR read %s failed, err: %v\n", *wasmFileFlag, err)
		return
	}
	byteCode = bytes.Trim(byteCode, "\"\r\n")
	code, err := hex.DecodeString(string(byteCode[2:])) // delete 0x
	if err != nil {
		fmt.Printf("ERR hex.DecodeString failed, byteCode file %s, err: %s\n", *wasmFileFlag, err)
		return
	}
	fmt.Printf("INFO code[%d]: %s\n", len(code), string(byteCode))

	initInput := []byte("Init|{}")
	fmt.Printf("INFO initInput[%d]: 0x%s [%s]\n",
		len(initInput), hex.EncodeToString(initInput), string(initInput))

	caller := MockAccountRef(testAddr1)
	to := MockAccountRef(testAddr2)
	value := big.NewInt(0).SetUint64(*contractValue)

	contract := vm.NewContract(caller.Address().Bytes(), to.Address().Bytes(), value, *contractGas)
	contract.SetCallCode(types.EmptyAddress.Bytes(), types.Keccak256Hash(code).Bytes(), code)
	contract.Input = initInput
	contract.CreateCall = true

	testHeader := types.Header{}
	ctx := wasm.NewWASMContext(&testHeader, &MockChainContext{}, &types.EmptyAddress, testGasRate)
	ctx.Time = testTime
	ctx.Origin = types.EmptyAddress
	ctx.GasPrice = testGasPrice

	st, _ := state.New()
	st.AddBalance(caller.Address(), testBalance1)
	st.AddBalance(to.Address(), testBalance2)

	// info := vm.ContractInfo{
	// 	Type: "wasm",
	// 	Path: "/tmp/aots/0x658294a3cdbad2ace0f633f4c00b3892523d6d05.so",
	// }
	// md5, _ := hex.DecodeString("f99e6f7ab4b44a6bb77a0fd2f2aafe05")
	// copy(info.MD5[:], md5[:16])
	// fmt.Printf("ContractInfo: %v\n", info)
	// infoData, _ := json.Marshal(&info)
	// st.SetContractInfo(contract.Address().Bytes(), infoData)

	eng := vm.NewEngine(contract, contract.Gas, st, log.With("mod", "wasm"))
	eng.SetTrace(false)
	wasm.Inject(&ctx, st)

	start := time.Now()

	app, err := eng.NewApp(contract.Address().String(), contract.Code, false)
	if err != nil {
		fmt.Printf("ERR vm/Engine.NewApp failed, err: %s\n", err)
		return
	}

	parseTime := time.Since(start).Seconds()

	fnIndex := app.GetExportFunction(vm.APPEntry)
	if fnIndex < 0 {
		fmt.Printf("INFO vm/APP.GetExportFunction, func=%s not exist\n", vm.APPEntry)
		return
	}
	app.EntryFunc = vm.APPEntry

	// go func() {
	// 	http.ListenAndServe(":8000", nil)
	// }()

	ret, err := eng.Run(app, contract.Input)
	if err != nil {
		fmt.Printf("ERR init vm/Engine.Run failed, func=%s gasUsed=%d gasLeft=%d, err: %s",
			vm.APPEntry, eng.GasUsed(), eng.Gas(), err)
		return
	}

	vmem := app.VM.VMemory()
	rBytes, err := vmem.GetString(ret)
	if err != nil {
		fmt.Printf("ERR init vm/MemManager.GetBytes failed, err: %v", err)
		return
	}

	fmt.Printf("Waiting gcc compiler...\n")
	time.Sleep(time.Second * 4)

	initTime := time.Since(start).Seconds()

	fmt.Printf("INFO init done, gasUsed=%d gasLeft=%d time=[%f:%f], return[%d]: %s\n",
		eng.GasUsed(), eng.Gas(), parseTime, initTime-parseTime, len(rBytes), string(rBytes))

	if len(*callFuncFlag) == 0 {
		fmt.Println("INFO init finished. You can provide the called function and data via parameter call")
		return
	}

	callInput, err := ioutil.ReadFile(*callFuncFlag)
	if err != nil {
		if !strings.Contains(*callFuncFlag, "|{") {
			fmt.Printf("ERR read %s failed, err: %s\n", *callFuncFlag, err)
			return
		}
		callInput = []byte(*callFuncFlag)
	}
	callInput = bytes.Trim(callInput, "\r\n")
	fmt.Printf("INFO callInput[%d]: 0x%s [%s]\n",
		len(callInput), hex.EncodeToString(callInput), string(callInput))

	contract.Input = callInput
	contract.CreateCall = false

	start = time.Now()

	app, err = eng.NewApp(contract.Address().String(), contract.Code, false)
	if err != nil {
		fmt.Printf("ERR vm/Engine.NewApp failed, err: %s\n", err)
		return
	}
	ret, err = eng.Run(app, contract.Input)
	if err != nil {
		fmt.Printf("ERR call vm/Engine.Run failed, func=%s gasUsed=%d gasLeft=%d, err: %s",
			vm.APPEntry, eng.GasUsed(), eng.Gas(), err)
		return
	}

	vmem = app.VM.VMemory()
	rBytes, err = vmem.GetString(ret)
	if err != nil {
		fmt.Printf("ERR call vm/MemManager.GetBytes failed, err: %v", err)
		return
	}

	callTime := time.Since(start).Seconds()

	fmt.Printf("INFO call done, gasUsed=%d gasLeft=%d time=[%f], return[%d]: %s\n",
		eng.GasUsed(), eng.Gas(), callTime, len(rBytes), string(rBytes))

	return
}
