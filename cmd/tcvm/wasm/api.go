package wasm

import (
	"fmt"
	"math/big"

	"github.com/xunleichain/tc-wasm/mock/deps/secp256k1"
	"github.com/xunleichain/tc-wasm/mock/types"
	"github.com/xunleichain/tc-wasm/vm"
	"golang.org/x/crypto/sha3"
)

var (
	ctx *Context
	db  types.StateDB
)

func init() {
	env := vm.NewEnvTable()

	env.RegisterFunc("TC_StorageSet", &TCStorageSet{}) //removed
	env.RegisterFunc("TC_StorageGet", &TCStorageGet{}) //removed

	env.RegisterFunc("TC_StorageSetString", &TCStorageSet{})
	env.RegisterFunc("TC_StorageSetBytes", &TCStorageSetBytes{})
	env.RegisterFunc("TC_StoragePureSetString", &TCStoragePureSetString{})
	env.RegisterFunc("TC_StoragePureSetBytes", &TCStoragePureSetBytes{})
	env.RegisterFunc("TC_StorageGetString", &TCStorageGet{})
	env.RegisterFunc("TC_StorageGetBytes", &TCStorageGet{})
	env.RegisterFunc("TC_StoragePureGetString", &TCStoragePureGet{})
	env.RegisterFunc("TC_StoragePureGetBytes", &TCStoragePureGet{})

	env.RegisterFunc("TC_StorageDel", &TCStorageDel{})
	env.RegisterFunc("TC_ContractStorageGet", &TCContractStorageGet{})
	env.RegisterFunc("TC_ContractStoragePureGet", &TCContractStoragePureGet{})
	env.RegisterFunc("TC_Notify", &TCNotify{})
	env.RegisterFunc("TC_BlockHash", &TCBlockHash{})
	env.RegisterFunc("TC_GetCoinbase", &TCGetCoinbase{})
	env.RegisterFunc("TC_GetGasLimit", &TCGetGasLimit{})
	env.RegisterFunc("TC_GetNumber", &TCGetNumber{})
	env.RegisterFunc("TC_Now", &TCNow{})
	env.RegisterFunc("TC_GetTxGasPrice", &TCGetTxGasPrice{})
	env.RegisterFunc("TC_GetTxOrigin", &TCGetTxOrigin{})
	env.RegisterFunc("TC_Log0", &TCLog0{})
	env.RegisterFunc("TC_Log1", &TCLog1{})
	env.RegisterFunc("TC_Log2", &TCLog2{})
	env.RegisterFunc("TC_Log3", &TCLog3{})
	env.RegisterFunc("TC_Log4", &TCLog4{})
	env.RegisterFunc("TC_SelfDestruct", &TCSelfDestruct{})
	env.RegisterFunc("TC_GetBalance", &TCGetBalance{})
	env.RegisterFunc("TC_CheckSign", new(TCCheckSign))
	env.RegisterFunc("TC_Ecrecover", new(TCEcrecover))

	env.RegisterFunc("TC_Issue", &TCIssue{})
	env.RegisterFunc("TC_Transfer", &TCTransfer{})
	env.RegisterFunc("TC_TransferToken", &TCTransferToken{})
	env.RegisterFunc("TC_TokenBalance", &TCTokenBalance{})
	env.RegisterFunc("TC_TokenAddress", &TCTokenAddress{})
	env.RegisterFunc("TC_GetMsgValue", &TCGetMsgValue{})
	env.RegisterFunc("TC_GetMsgTokenValue", &TCGetMsgTokenValue{})
}

func Inject(context *Context, stateDB types.StateDB) {
	ctx = context
	db = stateDB
}

type TCNotify struct{}

func (t *TCNotify) Call(index int64, ops interface{}, args []uint64) (uint64, error) {
	eng := ops.(*vm.Engine)
	return tcNotify(eng, index, args)
}
func (t *TCNotify) Gas(index int64, ops interface{}, args []uint64) (uint64, error) {
	eng := ops.(*vm.Engine)
	return vm.GasNotify(eng, index, args)
}

//c: void TC_Notify(char* eventID, char* data)
func tcNotify(eng *vm.Engine, index int64, args []uint64) (uint64, error) {
	runningFrame, _ := eng.RunningAppFrame()
	vmem := runningFrame.VM.VMemory()
	eventID, err := vmem.GetString(args[0])
	if err != nil {
		return 0, vm.ErrInvalidApiArgs
	}
	d := sha3.NewLegacyKeccak256()
	d.Write(eventID)
	eventIDHash := types.BytesToHash(d.Sum(nil))
	dataTmp, err := vmem.GetString(args[1])
	if err != nil {
		return 0, vm.ErrInvalidApiArgs
	}
	data := make([]byte, len(dataTmp))
	copy(data, dataTmp)

	topics := make([]types.Hash, 0)
	topics = append(topics, eventIDHash)
	db.AddLog(&types.Log{
		Address:     eng.Contract.Self.Address(),
		Topics:      topics,
		Data:        data,
		BlockNumber: ctx.BlockNumber.Uint64(),
		BlockTime:   ctx.Time.Uint64(),
	})
	return 0, nil
}

type TCStorageSetBytes struct{}

func (t *TCStorageSetBytes) Call(index int64, ops interface{}, args []uint64) (uint64, error) {
	eng := ops.(*vm.Engine)
	return tcStorageSetBytes(eng, index, args)
}
func (t *TCStorageSetBytes) Gas(index int64, ops interface{}, args []uint64) (uint64, error) {
	eng := ops.(*vm.Engine)
	return vm.GasStorageSetBytes(eng, index, args)
}

//c: void TC_StorageSetBytes(const char* key, const uint8_t* val, uint32_t size);
func tcStorageSetBytes(eng *vm.Engine, index int64, args []uint64) (uint64, error) {
	app, _ := eng.RunningAppFrame()
	vmem := app.VM.VMemory()
	key, err := vmem.GetString(args[0])
	if err != nil {
		return 0, vm.ErrMemoryGet
	}
	val, err := vmem.GetBytes(args[1], int(args[2]))
	if err != nil {
		return 0, vm.ErrMemoryGet
	}
	eng.Logger().Debug("TC_StorageSetBytes", "key", string(key), "val", string(val), "size", len(val))

	db.SetState(eng.Contract.Address(), types.Keccak256Hash(key), val)
	return 0, nil
}

type TCStoragePureSetString struct{}

func (t *TCStoragePureSetString) Call(index int64, ops interface{}, args []uint64) (uint64, error) {
	eng := ops.(*vm.Engine)
	return tcStoragePureSetString(eng, index, args)
}
func (t *TCStoragePureSetString) Gas(index int64, ops interface{}, args []uint64) (uint64, error) {
	eng := ops.(*vm.Engine)
	return vm.GasStoragePureSetString(eng, index, args)
}

//c:void TC_StoragePureSetString(const uint8_t* key, uint32_t size1, const char* val);
func tcStoragePureSetString(eng *vm.Engine, index int64, args []uint64) (uint64, error) {
	app, _ := eng.RunningAppFrame()
	vmem := app.VM.VMemory()
	key, err := vmem.GetBytes(args[0], int(args[1]))
	if err != nil {
		return 0, vm.ErrMemoryGet
	}
	val, err := vmem.GetString(args[2])
	if err != nil {
		return 0, vm.ErrMemoryGet
	}
	eng.Logger().Debug("TC_StoragePureSetString", "key", string(key), "size", len(key), "val", string(val), "size", len(val))

	db.SetState(eng.Contract.Address(), types.Keccak256Hash(key), val)
	return 0, nil
}

type TCStoragePureSetBytes struct{}

func (t *TCStoragePureSetBytes) Call(index int64, ops interface{}, args []uint64) (uint64, error) {
	eng := ops.(*vm.Engine)
	return tcStoragePureSetBytes(eng, index, args)
}
func (t *TCStoragePureSetBytes) Gas(index int64, ops interface{}, args []uint64) (uint64, error) {
	eng := ops.(*vm.Engine)
	return vm.GasStoragePureSetBytes(eng, index, args)
}

//c: void TC_StoragePureSetBytes(const uint8_t* key, uint32_t size1, const uint8_t* val, uint32_t size2);
func tcStoragePureSetBytes(eng *vm.Engine, index int64, args []uint64) (uint64, error) {
	app, _ := eng.RunningAppFrame()
	vmem := app.VM.VMemory()
	key, err := vmem.GetBytes(args[0], int(args[1]))
	if err != nil {
		return 0, vm.ErrMemoryGet
	}
	val, err := vmem.GetBytes(args[2], int(args[3]))
	if err != nil {
		return 0, vm.ErrMemoryGet
	}
	eng.Logger().Debug("TC_StoragePureSetBytes", "key", string(key), "size", len(key), "val", val, "size", len(val))

	db.SetState(eng.Contract.Address(), types.Keccak256Hash(key), val)
	return 0, nil
}

type TCStoragePureGet struct{}

func (t *TCStoragePureGet) Call(index int64, ops interface{}, args []uint64) (uint64, error) {
	eng := ops.(*vm.Engine)
	return tcStoragePureGet(eng, index, args)
}
func (t *TCStoragePureGet) Gas(index int64, ops interface{}, args []uint64) (uint64, error) {
	eng := ops.(*vm.Engine)
	return vm.GasStoragePureGet(eng, index, args)
}

//char* TC_StoragePureGetString(const uint8_t* key, uint32_t size);
//uint8_t* TC_StoragePureGetBytes(const uint8_t* key, uint32_t size);
func tcStoragePureGet(eng *vm.Engine, index int64, args []uint64) (uint64, error) {
	app, _ := eng.RunningAppFrame()
	vmem := app.VM.VMemory()
	key, err := vmem.GetBytes(args[0], int(args[1]))
	if err != nil {
		return 0, vm.ErrMemoryGet
	}

	val := db.GetState(eng.Contract.Address(), types.Keccak256Hash(key))
	valPointer, err := vmem.SetBytes(val)
	if err != nil {
		return 0, vm.ErrMemorySet
	}
	eng.Logger().Debug("TC_StoragePureGet", "key", string(key), "val", val, "size", len(val))

	return uint64(valPointer), nil
}

type TCStorageGet struct{}

func (t *TCStorageGet) Call(index int64, ops interface{}, args []uint64) (uint64, error) {
	eng := ops.(*vm.Engine)
	return tcStorageGet(eng, index, args)
}
func (t *TCStorageGet) Gas(index int64, ops interface{}, args []uint64) (uint64, error) {
	eng := ops.(*vm.Engine)
	return vm.GasStorageGet(eng, index, args)
}

// c: char * TC_StorageGet(char *key) removed
//char* TC_StorageGetString(const char* key);
//uint8_t* TC_StorageGetBytes(const char* key);
func tcStorageGet(eng *vm.Engine, index int64, args []uint64) (uint64, error) {
	app, _ := eng.RunningAppFrame()
	vmem := app.VM.VMemory()
	key, err := vmem.GetString(args[0])
	if err != nil {
		return 0, vm.ErrMemoryGet
	}

	val := db.GetState(eng.Contract.Address(), types.Keccak256Hash(key))
	valPointer, err := vmem.SetBytes(val)
	if err != nil {
		return 0, vm.ErrMemorySet
	}
	eng.Logger().Debug("TC_StorageGet", "key", string(key), "val", string(val), "size", len(val))

	return uint64(valPointer), nil
}

type TCContractStorageGet struct{}

func (t *TCContractStorageGet) Call(index int64, ops interface{}, args []uint64) (uint64, error) {
	eng := ops.(*vm.Engine)
	return tcContractStorageGet(eng, index, args)
}
func (t *TCContractStorageGet) Gas(index int64, ops interface{}, args []uint64) (uint64, error) {
	eng := ops.(*vm.Engine)
	return vm.GasContractStorageGet(eng, index, args)
}

// c: char * TC_ContractStorageGet(address contract, char *key)
func tcContractStorageGet(eng *vm.Engine, index int64, args []uint64) (uint64, error) {
	app, _ := eng.RunningAppFrame()
	vmem := app.VM.VMemory()
	contract, err := vmem.GetString(args[0])
	if err != nil {
		return 0, vm.ErrMemoryGet
	}

	key, err := vmem.GetString(args[1])
	if err != nil {
		return 0, vm.ErrMemoryGet
	}

	val := db.GetState(types.HexToAddress(string(contract)), types.Keccak256Hash(key))
	valPointer, err := vmem.SetBytes(val)
	if err != nil {
		return 0, vm.ErrMemorySet
	}
	eng.Logger().Debug("TC_ContractStorageGet", "key", string(key), "val", string(val))

	return uint64(valPointer), nil
}

type TCContractStoragePureGet struct{}

func (t *TCContractStoragePureGet) Call(index int64, ops interface{}, args []uint64) (uint64, error) {
	eng := ops.(*vm.Engine)
	return tcContractStoragePureGet(eng, index, args)
}

func (t *TCContractStoragePureGet) Gas(index int64, ops interface{}, args []uint64) (uint64, error) {
	eng := ops.(*vm.Engine)
	return vm.GasContractStoragePureGet(eng, index, args)
}

// c: char * TC_ContractStoragePureGet(address contract, uint8_t* key, uint32_t size)
func tcContractStoragePureGet(eng *vm.Engine, index int64, args []uint64) (uint64, error) {
	app, _ := eng.RunningAppFrame()
	vmem := app.VM.VMemory()
	contract, err := vmem.GetString(args[0])
	if err != nil {
		return 0, vm.ErrMemoryGet
	}
	key, err := vmem.GetBytes(args[1], int(args[2]))
	if err != nil {
		return 0, vm.ErrMemoryGet
	}

	// mState := eng.State.(types.StateDB)
	val := db.GetState(types.HexToAddress(string(contract)), types.Keccak256Hash(key))
	valPointer, err := vmem.SetBytes(val)
	if err != nil {
		return 0, vm.ErrMemorySet
	}
	eng.Logger().Debug("TC_ContractStoragePureGet", "key", string(key), "val", string(val))

	return uint64(valPointer), nil
}

type TCStorageSet struct{}

func (t *TCStorageSet) Call(index int64, ops interface{}, args []uint64) (uint64, error) {
	eng := ops.(*vm.Engine)
	return tcStorageSet(eng, index, args)
}

func (t *TCStorageSet) Gas(index int64, ops interface{}, args []uint64) (uint64, error) {
	eng := ops.(*vm.Engine)
	return vm.GasStorageSet(eng, index, args)
}

//c: void TC_StorageSetString(const char* key, const char* val);
func tcStorageSet(eng *vm.Engine, index int64, args []uint64) (uint64, error) {
	app, _ := eng.RunningAppFrame()
	vmem := app.VM.VMemory()
	key, err := vmem.GetString(args[0])
	if err != nil {
		return 0, vm.ErrMemoryGet
	}
	val, err := vmem.GetString(args[1])
	if err != nil {
		return 0, vm.ErrMemoryGet
	}
	eng.Logger().Debug("TC_StorageSetString", "key", string(key), "val", string(val))

	db.SetState(eng.Contract.Address(), types.Keccak256Hash(key), val)
	return 0, nil
}

type TCStorageDel struct{}

func (t *TCStorageDel) Call(index int64, ops interface{}, args []uint64) (uint64, error) {
	eng := ops.(*vm.Engine)
	return tcStorageDel(eng, index, args)
}
func (t *TCStorageDel) Gas(index int64, ops interface{}, args []uint64) (uint64, error) {
	eng := ops.(*vm.Engine)
	return vm.GasStorageDel(eng, index, args)
}

// c: void TC_StorageDel(char *key)
func tcStorageDel(eng *vm.Engine, index int64, args []uint64) (uint64, error) {
	app, _ := eng.RunningAppFrame()
	vmem := app.VM.VMemory()
	key, err := vmem.GetString(args[0])
	if err != nil {
		return 0, vm.ErrMemoryGet
	}
	eng.Logger().Debug("TC_StorageDel", "key", string(key))

	var val []byte
	db.SetState(eng.Contract.Address(), types.Keccak256Hash(key), val)
	return 0, nil
}

type TCBlockHash struct{}

func (t *TCBlockHash) Call(index int64, ops interface{}, args []uint64) (uint64, error) {
	eng := ops.(*vm.Engine)
	return tcBlockHash(eng, index, args)
}
func (t *TCBlockHash) Gas(index int64, ops interface{}, args []uint64) (uint64, error) {
	eng := ops.(*vm.Engine)
	return vm.GasBlockHash(eng, index, args)
}

//char *TC_blockhash(long long blockNumber)
func tcBlockHash(eng *vm.Engine, index int64, args []uint64) (uint64, error) {
	if len(args) != 1 {
		return 0, vm.ErrInvalidApiArgs
	}
	block := args[0]
	app, _ := eng.RunningAppFrame()
	vmem := app.VM.VMemory()
	hash := ctx.GetHash(block)
	hashStr := hash.String()

	hashPointer, err := vmem.SetBytes([]byte(hashStr))
	if err != nil {
		return 0, vm.ErrMemorySet
	}

	return uint64(hashPointer), nil
}

type TCGetCoinbase struct{}

func (t *TCGetCoinbase) Call(index int64, ops interface{}, args []uint64) (uint64, error) {
	eng := ops.(*vm.Engine)
	return tcGetCoinbase(eng, index, args)
}
func (t *TCGetCoinbase) Gas(index int64, ops interface{}, args []uint64) (uint64, error) {
	eng := ops.(*vm.Engine)
	return vm.GasGetCoinbase(eng, index, args)
}

// char *TC_get_coinbase()
func tcGetCoinbase(eng *vm.Engine, index int64, args []uint64) (uint64, error) {
	if len(args) != 0 {
		return 0, vm.ErrInvalidApiArgs
	}
	app, _ := eng.RunningAppFrame()
	vmem := app.VM.VMemory()
	coinbase := ctx.Coinbase
	coinbaseStr := coinbase.String()

	cPointer, err := vmem.SetBytes([]byte(coinbaseStr))
	if err != nil {
		return 0, vm.ErrMemorySet
	}

	return uint64(cPointer), nil
}

type TCGetGasLimit struct{}

func (t *TCGetGasLimit) Call(index int64, ops interface{}, args []uint64) (uint64, error) {
	eng := ops.(*vm.Engine)
	return tcGetGasLimit(eng, index, args)
}
func (t *TCGetGasLimit) Gas(index int64, ops interface{}, args []uint64) (uint64, error) {
	eng := ops.(*vm.Engine)
	return vm.GasGetGasLimit(eng, index, args)
}

// long long TC_get_gaslimit()
func tcGetGasLimit(eng *vm.Engine, index int64, args []uint64) (uint64, error) {
	if len(args) != 0 {
		return 0, vm.ErrInvalidApiArgs
	}
	gaslimit := ctx.GasLimit

	return uint64(gaslimit), nil
}

type TCGetNumber struct{}

func (t *TCGetNumber) Call(index int64, ops interface{}, args []uint64) (uint64, error) {
	eng := ops.(*vm.Engine)
	return tcGetNumber(eng, index, args)
}
func (t *TCGetNumber) Gas(index int64, ops interface{}, args []uint64) (uint64, error) {
	eng := ops.(*vm.Engine)
	return vm.GasGetNumber(eng, index, args)
}

// long long TC_get_number()
func tcGetNumber(eng *vm.Engine, index int64, args []uint64) (uint64, error) {
	if len(args) != 0 {
		return 0, vm.ErrInvalidApiArgs
	}
	return ctx.BlockNumber.Uint64(), nil
}

type TCGetTimestamp struct{}

func (t *TCGetTimestamp) Call(index int64, ops interface{}, args []uint64) (uint64, error) {
	eng := ops.(*vm.Engine)
	return tcGetTimestamp(eng, index, args)
}
func (t *TCGetTimestamp) Gas(index int64, ops interface{}, args []uint64) (uint64, error) {
	eng := ops.(*vm.Engine)
	return vm.GasGetTimestamp(eng, index, args)
}

// long long TC_get_timestamp()
func tcGetTimestamp(eng *vm.Engine, index int64, args []uint64) (uint64, error) {
	if len(args) != 0 {
		return 0, vm.ErrInvalidApiArgs
	}
	return ctx.Time.Uint64(), nil
}

type TCNow struct{}

func (t *TCNow) Call(index int64, ops interface{}, args []uint64) (uint64, error) {
	eng := ops.(*vm.Engine)
	return tcNow(eng, index, args)
}
func (t *TCNow) Gas(index int64, ops interface{}, args []uint64) (uint64, error) {
	eng := ops.(*vm.Engine)
	return vm.GasNow(eng, index, args)
}

// long long TC_now()
func tcNow(eng *vm.Engine, index int64, args []uint64) (uint64, error) {
	if len(args) != 0 {
		return 0, vm.ErrInvalidApiArgs
	}
	return ctx.Time.Uint64(), nil
}

type TCGetTxGasPrice struct{}

func (t *TCGetTxGasPrice) Call(index int64, ops interface{}, args []uint64) (uint64, error) {
	eng := ops.(*vm.Engine)
	return tcGetTxGasPrice(eng, index, args)
}
func (t *TCGetTxGasPrice) Gas(index int64, ops interface{}, args []uint64) (uint64, error) {
	eng := ops.(*vm.Engine)
	return vm.GasGetTxGasPrice(eng, index, args)
}

// long long TC_get_tx_gasprice()
func tcGetTxGasPrice(eng *vm.Engine, index int64, args []uint64) (uint64, error) {
	if len(args) != 0 {
		return 0, vm.ErrInvalidApiArgs
	}
	return ctx.GasPrice.Uint64(), nil
}

type TCGetTxOrigin struct{}

func (t *TCGetTxOrigin) Call(index int64, ops interface{}, args []uint64) (uint64, error) {
	eng := ops.(*vm.Engine)
	return tcGetTxOrigin(eng, index, args)
}
func (t *TCGetTxOrigin) Gas(index int64, ops interface{}, args []uint64) (uint64, error) {
	eng := ops.(*vm.Engine)
	return vm.GasGetTxOrigin(eng, index, args)
}

// char *TC_get_tx_origin()
func tcGetTxOrigin(eng *vm.Engine, index int64, args []uint64) (uint64, error) {
	if len(args) != 0 {
		return 0, vm.ErrInvalidApiArgs
	}
	app, _ := eng.RunningAppFrame()
	vmem := app.VM.VMemory()
	orign := ctx.Origin

	dataPtr, err := vmem.SetBytes([]byte(orign.String()))
	if err != nil {
		return 0, vm.ErrMemorySet
	}
	return uint64(dataPtr), nil
}

type TCGetBalance struct{}

func (t *TCGetBalance) Call(index int64, ops interface{}, args []uint64) (uint64, error) {
	eng := ops.(*vm.Engine)
	return tcGetBalance(eng, index, args)
}
func (t *TCGetBalance) Gas(index int64, ops interface{}, args []uint64) (uint64, error) {
	eng := ops.(*vm.Engine)
	return vm.GasGetBalance(eng, index, args)
}

//char* TC_GetBalance(char *address)
func tcGetBalance(eng *vm.Engine, index int64, args []uint64) (uint64, error) {
	runningFrame, _ := eng.RunningAppFrame()
	vmem := runningFrame.VM.VMemory()
	addrTmp, err := vmem.GetString(args[0])
	if err != nil || !types.IsHexAddress(string(addrTmp)) {
		return 0, vm.ErrInvalidApiArgs
	}
	addr := types.HexToAddress(string(addrTmp))
	balance := db.GetBalance(addr)
	eng.Logger().Debug("tcGetBalance", "balance", balance.String())
	return vmem.SetBytes([]byte(balance.String()))
}

type TCTransfer struct{}

func (t *TCTransfer) Call(index int64, ops interface{}, args []uint64) (uint64, error) {
	eng := ops.(*vm.Engine)
	return tcTransfer(eng, index, args)
}
func (t *TCTransfer) Gas(index int64, ops interface{}, args []uint64) (uint64, error) {
	eng := ops.(*vm.Engine)
	return vm.GasTransfer(eng, index, args)
}

//void TC_Transfer(char *address, char* amount)
func tcTransfer(eng *vm.Engine, index int64, args []uint64) (uint64, error) {
	runningFrame, _ := eng.RunningAppFrame()
	vmem := runningFrame.VM.VMemory()
	from := eng.Contract.Self.Address()
	toTmp, err := vmem.GetString(args[0])
	if err != nil || !types.IsHexAddress(string(toTmp)) {
		return 0, vm.ErrInvalidApiArgs
	}
	to := types.HexToAddress(string(toTmp))
	valTmp, err := vmem.GetString(args[1])
	if err != nil {
		return 0, vm.ErrInvalidApiArgs
	}
	val, ok := big.NewInt(0).SetString(string(valTmp), 0)
	if !ok || val.Sign() < 0 {
		return 0, vm.ErrInvalidApiArgs
	}

	eng.Logger().Debug("tcTransfer", "from", from.String(), "to", to.String(), "val", val)
	if val.Sign() == 0 {
		return 0, nil
	}
	if db.GetBalance(from).Cmp(val) < 0 {
		return 0, vm.ErrBalanceNotEnough
	}
	db.SubBalance(from, val)
	db.AddBalance(to, val)

	return 0, nil
}

type TCTransferToken struct{}

func (t *TCTransferToken) Call(index int64, ops interface{}, args []uint64) (uint64, error) {
	eng := ops.(*vm.Engine)
	return tcTransferToken(eng, index, args)
}
func (t *TCTransferToken) Gas(index int64, ops interface{}, args []uint64) (uint64, error) {
	eng := ops.(*vm.Engine)
	return vm.GasTransferToken(eng, index, args)
}

//void TC_TransferToken(char *address, char* tokenAddress, char* amount)
func tcTransferToken(eng *vm.Engine, index int64, args []uint64) (uint64, error) {
	runningFrame, _ := eng.RunningAppFrame()
	vmem := runningFrame.VM.VMemory()
	from := eng.Contract.Self.Address()
	toTmp, err := vmem.GetString(args[0])
	if err != nil || !types.IsHexAddress(string(toTmp)) {
		return 0, vm.ErrInvalidApiArgs
	}
	to := types.HexToAddress(string(toTmp))
	tokenTmp, err := vmem.GetString(args[1])
	if err != nil || !types.IsHexAddress(string(tokenTmp)) {
		return 0, vm.ErrInvalidApiArgs
	}
	token := types.HexToAddress(string(tokenTmp))
	valTmp, err := vmem.GetString(args[2])
	if err != nil {
		return 0, vm.ErrInvalidApiArgs
	}
	val, ok := big.NewInt(0).SetString(string(valTmp), 0)
	if !ok || val.Sign() < 0 {
		return 0, vm.ErrInvalidApiArgs
	}

	eng.Logger().Debug("tcTransferToken", "from", from.String(), "to", to.String(), "token", token.String(), "val", val)
	if val.Sign() == 0 {
		return 0, nil
	}

	if token == types.EmptyAddress {
		if db.GetBalance(from).Cmp(val) >= 0 {
			db.SubBalance(from, val)
			db.AddBalance(to, val)
		} else {
			eng.Logger().Info("insufficient BaseToken balance")
			return 0, vm.ErrBalanceNotEnough
		}
	} else {
		if db.GetTokenBalance(from, token).Cmp(val) >= 0 {
			db.SubTokenBalance(from, token, val)
			db.AddTokenBalance(to, token, val)
		} else {
			eng.Logger().Info("insufficient token balance")
			return 0, vm.ErrBalanceNotEnough
		}
	}

	return 0, nil
}

type TCSelfDestruct struct{}

func (t *TCSelfDestruct) Call(index int64, ops interface{}, args []uint64) (uint64, error) {
	eng := ops.(*vm.Engine)
	return tcSelfDestruct(eng, index, args)
}
func (t *TCSelfDestruct) Gas(index int64, ops interface{}, args []uint64) (uint64, error) {
	eng := ops.(*vm.Engine)
	return vm.GasSelfDestruct(eng, index, args)
}

//char *TC_SelfDestruct(char* recipient)
func tcSelfDestruct(eng *vm.Engine, index int64, args []uint64) (uint64, error) {
	runningFrame, _ := eng.RunningAppFrame()
	vmem := runningFrame.VM.VMemory()
	addr := eng.Contract.Self.Address()
	toTmp, err := vmem.GetString(args[0])
	if err != nil || !types.IsHexAddress(string(toTmp)) {
		return 0, vm.ErrInvalidApiArgs
	}
	to := types.HexToAddress(string(toTmp))
	tv := db.GetTokenBalances(addr)

	for i := 0; i < len(tv); i++ {
		db.AddTokenBalance(to, tv[i].TokenAddr, tv[i].Value)
	}

	//suicideToken(eng, addr, to)
	db.Suicide(addr)
	//delete cache
	eng.RemoveCache(addr.String())
	// eng.AppCache.Delete(addr.String())

	return 0, nil
}

type TCLog0 struct{}

func (t *TCLog0) Call(index int64, ops interface{}, args []uint64) (uint64, error) {
	eng := ops.(*vm.Engine)
	return tcLog0(eng, index, args)
}
func (t *TCLog0) Gas(index int64, ops interface{}, args []uint64) (uint64, error) {
	eng := ops.(*vm.Engine)
	gasFunc := vm.MakeGasLog(0)
	return gasFunc(eng, index, args)
}

//void TC_Log0(char* data)
func tcLog0(eng *vm.Engine, index int64, args []uint64) (uint64, error) {
	runningFrame, _ := eng.RunningAppFrame()
	vmem := runningFrame.VM.VMemory()
	dataTmp, err := vmem.GetString(args[0])
	if err != nil {
		return 0, vm.ErrInvalidApiArgs
	}
	data := make([]byte, len(dataTmp))
	copy(data, dataTmp)

	topics := make([]types.Hash, 0)
	db.AddLog(&types.Log{
		Address:     eng.Contract.Self.Address(),
		Topics:      topics,
		Data:        data,
		BlockNumber: ctx.BlockNumber.Uint64(),
		BlockTime:   ctx.Time.Uint64(),
	})
	return 0, nil
}

type TCLog1 struct{}

func (t *TCLog1) Call(index int64, ops interface{}, args []uint64) (uint64, error) {
	eng := ops.(*vm.Engine)
	return tcLog1(eng, index, args)
}
func (t *TCLog1) Gas(index int64, ops interface{}, args []uint64) (uint64, error) {
	eng := ops.(*vm.Engine)
	gasFunc := vm.MakeGasLog(1)
	return gasFunc(eng, index, args)
}

//void TC_Log1(char* data, char* topic)
func tcLog1(eng *vm.Engine, index int64, args []uint64) (uint64, error) {
	runningFrame, _ := eng.RunningAppFrame()
	vmem := runningFrame.VM.VMemory()
	dataTmp, err := vmem.GetString(args[0])
	if err != nil {
		return 0, vm.ErrInvalidApiArgs
	}
	data := make([]byte, len(dataTmp))
	copy(data, dataTmp)

	topics := make([]types.Hash, 0)
	for i := 1; i < 2; i++ {
		topicTmp, err := vmem.GetString(args[i])
		if err != nil {
			return 0, vm.ErrInvalidApiArgs
		}
		topic := types.BytesToHash(topicTmp)
		topics = append(topics, topic)
	}
	db.AddLog(&types.Log{
		Address:     eng.Contract.Self.Address(),
		Topics:      topics,
		Data:        data,
		BlockNumber: ctx.BlockNumber.Uint64(),
		BlockTime:   ctx.Time.Uint64(),
	})
	return 0, nil
}

type TCLog2 struct{}

func (t *TCLog2) Call(index int64, ops interface{}, args []uint64) (uint64, error) {
	eng := ops.(*vm.Engine)
	return tcLog2(eng, index, args)
}
func (t *TCLog2) Gas(index int64, ops interface{}, args []uint64) (uint64, error) {
	eng := ops.(*vm.Engine)
	gasFunc := vm.MakeGasLog(2)
	return gasFunc(eng, index, args)
}

//void TC_Log2(char* data, char* topic1, char* topic2)
func tcLog2(eng *vm.Engine, index int64, args []uint64) (uint64, error) {
	runningFrame, _ := eng.RunningAppFrame()
	vmem := runningFrame.VM.VMemory()
	dataTmp, err := vmem.GetString(args[0])
	if err != nil {
		return 0, vm.ErrInvalidApiArgs
	}
	data := make([]byte, len(dataTmp))
	copy(data, dataTmp)

	topics := make([]types.Hash, 0)
	for i := 1; i < 3; i++ {
		topicTmp, err := vmem.GetString(args[i])
		if err != nil {
			return 0, vm.ErrInvalidApiArgs
		}
		topic := types.BytesToHash(topicTmp)
		topics = append(topics, topic)
	}
	db.AddLog(&types.Log{
		Address:     eng.Contract.Self.Address(),
		Topics:      topics,
		Data:        data,
		BlockNumber: ctx.BlockNumber.Uint64(),
		BlockTime:   ctx.Time.Uint64(),
	})
	return 0, nil
}

type TCLog3 struct{}

func (t *TCLog3) Call(index int64, ops interface{}, args []uint64) (uint64, error) {
	eng := ops.(*vm.Engine)
	return tcLog3(eng, index, args)
}
func (t *TCLog3) Gas(index int64, ops interface{}, args []uint64) (uint64, error) {
	eng := ops.(*vm.Engine)
	gasFunc := vm.MakeGasLog(3)
	return gasFunc(eng, index, args)
}

//void TC_Log3(char* data, char* topic1, char* topic2, char* topic3)
func tcLog3(eng *vm.Engine, index int64, args []uint64) (uint64, error) {
	runningFrame, _ := eng.RunningAppFrame()
	vmem := runningFrame.VM.VMemory()
	dataTmp, err := vmem.GetString(args[0])
	if err != nil {
		return 0, vm.ErrInvalidApiArgs
	}
	data := make([]byte, len(dataTmp))
	copy(data, dataTmp)

	topics := make([]types.Hash, 0)
	for i := 1; i < 4; i++ {
		topicTmp, err := vmem.GetString(args[i])
		if err != nil {
			return 0, vm.ErrInvalidApiArgs
		}
		topic := types.BytesToHash(topicTmp)
		topics = append(topics, topic)
	}
	db.AddLog(&types.Log{
		Address:     eng.Contract.Self.Address(),
		Topics:      topics,
		Data:        data,
		BlockNumber: ctx.BlockNumber.Uint64(),
		BlockTime:   ctx.Time.Uint64(),
	})
	return 0, nil
}

type TCLog4 struct{}

func (t *TCLog4) Call(index int64, ops interface{}, args []uint64) (uint64, error) {
	eng := ops.(*vm.Engine)
	return tcLog4(eng, index, args)
}
func (t *TCLog4) Gas(index int64, ops interface{}, args []uint64) (uint64, error) {
	eng := ops.(*vm.Engine)
	gasFunc := vm.MakeGasLog(4)
	return gasFunc(eng, index, args)
}

//void TC_Log4(char* data, char* topic1, char* topic2, char* topic3, char* topic4)
func tcLog4(eng *vm.Engine, index int64, args []uint64) (uint64, error) {
	runningFrame, _ := eng.RunningAppFrame()
	vmem := runningFrame.VM.VMemory()
	dataTmp, err := vmem.GetString(args[0])
	if err != nil {
		return 0, vm.ErrInvalidApiArgs
	}
	data := make([]byte, len(dataTmp))
	copy(data, dataTmp)

	topics := make([]types.Hash, 0)
	for i := 1; i < 5; i++ {
		topicTmp, err := vmem.GetString(args[i])
		if err != nil {
			return 0, vm.ErrInvalidApiArgs
		}
		topic := types.BytesToHash(topicTmp)
		topics = append(topics, topic)
	}
	db.AddLog(&types.Log{
		Address:     eng.Contract.Self.Address(),
		Topics:      topics,
		Data:        data,
		BlockNumber: ctx.BlockNumber.Uint64(),
		BlockTime:   ctx.Time.Uint64(),
	})
	return 0, nil
}

type TCIssue struct{}

func (t *TCIssue) Call(index int64, ops interface{}, args []uint64) (uint64, error) {
	eng := ops.(*vm.Engine)
	return tcIssue(eng, index, args)
}
func (t *TCIssue) Gas(index int64, ops interface{}, args []uint64) (uint64, error) {
	eng := ops.(*vm.Engine)
	return vm.GasIssue(eng, index, args)
}

//void TC_Issue(char* amount);
func tcIssue(eng *vm.Engine, index int64, args []uint64) (uint64, error) {
	runningFrame, _ := eng.RunningAppFrame()
	vmem := runningFrame.VM.VMemory()
	amountTmp, err := vmem.GetString(args[0])
	if err != nil {
		return 0, vm.ErrInvalidApiArgs
	}
	amount, ok := new(big.Int).SetString(string(amountTmp), 0)
	if !ok {
		return 0, vm.ErrInvalidApiArgs
	}

	if amount.Sign() > 0 {
		contractAddr := eng.Contract.Self.Address()
		db.AddTokenBalance(contractAddr, *eng.Contract.CodeAddr, amount)
	}

	return 0, nil
}

type TCTokenBalance struct{}

func (t *TCTokenBalance) Call(index int64, ops interface{}, args []uint64) (uint64, error) {
	eng := ops.(*vm.Engine)
	return tcTokenBalance(eng, index, args)
}
func (t *TCTokenBalance) Gas(index int64, ops interface{}, args []uint64) (uint64, error) {
	eng := ops.(*vm.Engine)
	return vm.GasTokenBalance(eng, index, args)
}

//char* TC_TokenBalance(char* addr, char* token);
func tcTokenBalance(eng *vm.Engine, index int64, args []uint64) (uint64, error) {
	runningFrame, _ := eng.RunningAppFrame()
	vmem := runningFrame.VM.VMemory()
	addrTmp, err := vmem.GetString(args[0])
	if err != nil || !types.IsHexAddress(string(addrTmp)) {
		return 0, vm.ErrInvalidApiArgs
	}
	addr := types.HexToAddress(string(addrTmp))
	tokenTmp, err := vmem.GetString(args[1])
	if err != nil || !types.IsHexAddress(string(tokenTmp)) {
		return 0, vm.ErrInvalidApiArgs
	}
	token := types.HexToAddress(string(tokenTmp))

	var balance *big.Int
	if token == types.EmptyAddress {
		balance = db.GetBalance(addr)
	} else {
		balance = db.GetTokenBalance(addr, token)
	}
	return vmem.SetBytes([]byte(balance.String()))
}

type TCTokenAddress struct{}

func (t *TCTokenAddress) Call(index int64, ops interface{}, args []uint64) (uint64, error) {
	eng := ops.(*vm.Engine)
	return tcTokenAddress(eng, index, args)
}
func (t *TCTokenAddress) Gas(index int64, ops interface{}, args []uint64) (uint64, error) {
	eng := ops.(*vm.Engine)
	return vm.GasTokenAddress(eng, index, args)
}

//char* TC_TokenAddress();
func tcTokenAddress(eng *vm.Engine, index int64, args []uint64) (uint64, error) {
	runningFrame, _ := eng.RunningAppFrame()
	vmem := runningFrame.VM.VMemory()
	if ctx.Token == types.EmptyAddress {
		return vmem.SetBytes([]byte(types.Address{}.String()))
	}
	return vmem.SetBytes([]byte(ctx.Token.String()))
}

type TCGetMsgValue struct{}

func (t *TCGetMsgValue) Call(index int64, ops interface{}, args []uint64) (uint64, error) {
	eng := ops.(*vm.Engine)
	return tcGetMsgValue(eng, index, args)
}
func (t *TCGetMsgValue) Gas(index int64, ops interface{}, args []uint64) (uint64, error) {
	eng := ops.(*vm.Engine)
	return gasGetMsgValue(eng, index, args)
}

// char *TC_get_msg_value()
func tcGetMsgValue(eng *vm.Engine, index int64, args []uint64) (uint64, error) {
	if len(args) != 0 {
		return 0, vm.ErrInvalidApiArgs
	}
	app, _ := eng.RunningAppFrame()
	vmem := app.VM.VMemory()
	vStr := eng.Contract.Value().String()
	var (
		dataPtr uint64
		err     error
	)
	if ctx.Token == types.EmptyAddress {
		dataPtr, err = vmem.SetBytes([]byte(vStr))
	} else {
		dataPtr, err = vmem.SetBytes([]byte(big.NewInt(0).String()))
	}
	if err != nil {
		return 0, vm.ErrMemorySet
	}
	return dataPtr, nil
}

func gasGetMsgValue(eng *vm.Engine, index int64, args []uint64) (uint64, error) {
	valLen := 1
	if ctx.Token == types.EmptyAddress {
		valLen = len(eng.Contract.Value().String())
	}
	gas := vm.GasExtStep
	wordGas, overflow := vm.SafeMul(vm.ToWordSize(uint64(valLen)), vm.CopyGas)
	if overflow {
		return 0, vm.ErrGasOverflow
	}
	if gas, overflow = vm.SafeAdd(gas, wordGas); overflow {
		return 0, vm.ErrGasOverflow
	}
	return gas, nil
}

type TCGetMsgTokenValue struct{}

func (t *TCGetMsgTokenValue) Call(index int64, ops interface{}, args []uint64) (uint64, error) {
	eng := ops.(*vm.Engine)
	return tcGetMsgTokenValue(eng, index, args)
}
func (t *TCGetMsgTokenValue) Gas(index int64, ops interface{}, args []uint64) (uint64, error) {
	eng := ops.(*vm.Engine)
	return gasGetMsgTokenValue(eng, index, args)
}

// char *TC_get_msg_token_value()
func tcGetMsgTokenValue(eng *vm.Engine, index int64, args []uint64) (uint64, error) {
	if len(args) != 0 {
		return 0, vm.ErrInvalidApiArgs
	}
	app, _ := eng.RunningAppFrame()
	vmem := app.VM.VMemory()
	vStr := eng.Contract.Value().String()
	var (
		dataPtr uint64
		err     error
	)
	if ctx.Token == types.EmptyAddress {
		dataPtr, err = vmem.SetBytes([]byte(big.NewInt(0).String()))
	} else {
		dataPtr, err = vmem.SetBytes([]byte(vStr))
	}
	if err != nil {
		return 0, vm.ErrMemorySet
	}
	return dataPtr, nil
}

func gasGetMsgTokenValue(eng *vm.Engine, index int64, args []uint64) (uint64, error) {
	valLen := len(eng.Contract.Value().String())
	if ctx.Token == types.EmptyAddress {
		valLen = 1
	}
	gas := vm.GasExtStep
	wordGas, overflow := vm.SafeMul(vm.ToWordSize(uint64(valLen)), vm.CopyGas)
	if overflow {
		return 0, vm.ErrGasOverflow
	}
	if gas, overflow = vm.SafeAdd(gas, wordGas); overflow {
		return 0, vm.ErrGasOverflow
	}
	return gas, nil
}

type TCCheckSign struct{}

func (t *TCCheckSign) Call(index int64, ops interface{}, args []uint64) (uint64, error) {
	eng := ops.(*vm.Engine)
	return tcCheckSign(eng, index, args)
}
func (t *TCCheckSign) Gas(index int64, ops interface{}, args []uint64) (uint64, error) {
	eng := ops.(*vm.Engine)
	return vm.GasCheckSign(eng, index, args)
}

//c: int TC_CheckSig(char * pubkey,char * data,char * sig)
func tcCheckSign(eng *vm.Engine, index int64, args []uint64) (uint64, error) {
	app, _ := eng.RunningAppFrame()
	vmem := app.VM.VMemory()

	arg0 := args[0]
	arg0_b, err := vmem.GetString(arg0)
	if err != nil {
		return 0, err
	}

	arg1 := args[1]
	arg1_b, err := vmem.GetString(arg1)
	if err != nil {
		return 0, err
	}

	arg2 := args[2]
	arg2_b, err := vmem.GetString(arg2)
	if err != nil {
		return 0, err
	}
	addr := types.HexToAddress(string(arg0_b))
	data := types.FromHex(string(arg1_b))
	sig := types.FromHex(string(arg2_b))
	pubkeyRecover, err := secp256k1.RecoverPubkey(data, sig)
	if err != nil {
		return 0, err
	}
	addrRecover := types.BytesToAddress(types.Keccak256(pubkeyRecover[1:])[12:])
	var ret uint64
	if addr == addrRecover {
		ret = 1
	} else {
		ret = 0
	}
	return uint64(ret), nil
}

type TCEcrecover struct{}

func (t *TCEcrecover) Call(index int64, ops interface{}, args []uint64) (uint64, error) {
	eng := ops.(*vm.Engine)
	return tcEcrecover(eng, index, args)
}
func (t *TCEcrecover) Gas(index int64, ops interface{}, args []uint64) (uint64, error) {
	eng := ops.(*vm.Engine)
	return vm.GasEcrecover(eng, index, args)
}

//char *TC_Ecrecover(char* hash, char* v, char* r, char* s)
func tcEcrecover(eng *vm.Engine, index int64, args []uint64) (uint64, error) {
	runningFrame, _ := eng.RunningAppFrame()
	vmem := runningFrame.VM.VMemory()
	hashTmp, herr := vmem.GetString(args[0])
	vTmp, verr := vmem.GetString(args[1])
	rTmp, rerr := vmem.GetString(args[2])
	sTmp, serr := vmem.GetString(args[3])
	if herr != nil || verr != nil || rerr != nil || serr != nil {
		return 0, vm.ErrInvalidApiArgs
	}
	hash := types.HexToHash(string(hashTmp))
	v, vok := new(big.Int).SetString(string(vTmp), 0)
	r, rok := new(big.Int).SetString(string(rTmp), 0)
	s, sok := new(big.Int).SetString(string(sTmp), 0)
	if !vok || !rok || !sok {
		return 0, vm.ErrInvalidApiArgs
	}
	sign := make([]byte, 65)
	copy(sign[:32], r.Bytes())
	copy(sign[32:64], s.Bytes())
	chainIdMul := new(big.Int).SetInt64(vm.THUNDERCHAINID * 2)
	sign[64] = byte(new(big.Int).Sub(v, chainIdMul).Uint64() - 35)
	// tighter sig s values input homestead only apply to tx sigs
	if !types.ValidateSignatureValues(sign[64], r, s, false) {
		return 0, vm.ErrInvalidApiArgs
	}
	// v needs to be at the end for libsecp256k1
	pubKey, err := secp256k1.RecoverPubkey(hash.Bytes(), sign)
	// make sure the public key is a valid one
	if err != nil {
		return 0, vm.ErrInvalidApiArgs
	}
	ret := fmt.Sprintf("0x%x", types.Keccak256(pubKey[1:])[12:])
	eng.Logger().Debug("tcEcrecover", "ret", ret)
	return vmem.SetBytes([]byte(ret))
}
