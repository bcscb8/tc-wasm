package vm

import (
	"bytes"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"sync"
	"time"

	"github.com/go-interpreter/wagon/exec"

	"github.com/xunleichain/tc-wasm/mock/types"
)

// AotService --
type AotService struct {
	path        string
	keepCSource bool
	exit        chan struct{}
	refresh     chan *APP

	black map[string]struct{}
	succ  map[string]*Native
	lock  sync.Mutex
}

// Env Variable
const TCVM_AOTS_ROOT = "TCVM_AOTS_ROOT"
const TCVM_AOTS_KEEP_CSOURCE = "TCVM_AOTS_KEEP_CSOURCE"

var aots *AotService

// NewAotService --
func NewAotService(path string, keepSrouce bool) *AotService {
	s := AotService{
		path:        path,
		keepCSource: keepSrouce,
		exit:        make(chan struct{}),
		refresh:     make(chan *APP, 8),
		black:       make(map[string]struct{}),
		succ:        make(map[string]*Native, 16),
	}

	return &s
}

// RefreshApp --
func RefreshApp(app *APP) {
	aots.checkApp(app)
}

// GetNative --
func GetNative(app *APP) *Native {
	return aots.getNative(app)
}

// StopAots --
func StopAots() {
	aots.exit <- struct{}{}
}

// ------------------------------------------------

// ContractInfo --
type ContractInfo struct {
	Type string   `json:"t"`
	Path string   `json:"p"`
	MD5  [16]byte `json:"md5"`
	Err  string   `json:"e"`
}

func (s *AotService) checkApp(app *APP) {
	if app.native == nil {
		select {
		case s.refresh <- app:
		default:
		}
	}
}

func (s *AotService) getNative(app *APP) *Native {
	s.lock.Lock()
	defer s.lock.Unlock()

	native := s.succ[app.Name]
	return native.clone(app)
}

func (s *AotService) loop() {
	d := time.Duration(time.Minute * 5)
	t := time.NewTimer(d)

	for {
		select {
		case app := <-s.refresh:
			if _, ok := s.black[app.Name]; ok {
				continue
			}

			s.lock.Lock()
			n := s.succ[app.Name]
			s.lock.Unlock()

			if n == nil {
				s.doCheck(app)
			}

		case <-t.C:
			cnt := 0
			now := time.Now()
			target := time.Unix(now.Unix()-3600, 0) // one hour

			s.lock.Lock()
			for name, native := range s.succ {
				if native.t.Before(target) {
					delete(s.succ, name)
					native.close()
					cnt++
					fmt.Printf("[AotService] delete native: %s\n", name)
				}
				if cnt >= 3 {
					break
				}
			}
			s.lock.Unlock()

			t.Reset(d)
		case <-s.exit:
			t.Stop()
			fmt.Printf("[AotService] Exit\n")
			return
		}
	}
}

func (s *AotService) doCheck(app *APP) error {
	if app.native != nil {
		return nil
	}

	info := s.getContractInfo(app)
	if info == nil {
		return s.doWork(app)
	}

	// @Note: now we only support wasm
	if info.Type != "wasm" {
		app.Printf("[AotService] Not wasm contract, skip it: app:%s", app.Name)
		return nil
	}

	if info.Err != "" {
		app.Printf("[AotService] ContractInfo Has Err: app:%s, err:%s", app.Name, info.Err)
		s.black[app.Name] = struct{}{}
		return fmt.Errorf(info.Err)
	}

	stat, err := os.Stat(info.Path)
	if err != nil {
		app.Printf("[AotService] os.Stat %s fail: app:%s, err:%s", info.Path, app.Name, err)
		if os.ErrNotExist == err {
			return s.doWork(app)
		}
		return err
	}
	if stat.IsDir() {
		app.Printf("[AotService] %s is dir, skip it: app:%s", info.Path, app.Name)
		if err = os.Remove(info.Path); err != nil {
			return err
		}
		return s.doWork(app)
	}

	data, err := ioutil.ReadFile(info.Path)
	if err != nil {
		app.Printf("[AotService] ReadFile %s fail: %s", info.Path, err)
		if err = os.Remove(info.Path); err != nil {
			return err
		}
		return s.doWork(app)
	}

	sum := md5.Sum(data)
	if !bytes.Equal(sum[:], info.MD5[:]) {
		app.Printf("[AotService] MD5 Not match: wanted=%s, goted=%s",
			hex.EncodeToString(info.MD5[:]), hex.EncodeToString(sum[:]))
		if err = os.Remove(info.Path); err != nil {
			return err
		}
		return s.doWork(app)
	}

	return s.doLoad(app, info)
}

func (s *AotService) doWork(app *APP) error {
	info, err := s.doCompile(app)
	if err != nil {
		app.Printf("[AotService] %s: app:%s, err:%s", info.Err, app.Name, err)
		s.updateContractInfo(app, info)
		return err
	}

	return s.doLoad(app, info)
}

func (s *AotService) doLoad(app *APP, info *ContractInfo) error {
	native, err := NewNative(app, info.Path)
	if err != nil {
		app.Printf("[AotService] NewNative fail: app:%s, err:%s", app.Name, err)
		info.Err = "NewNative Fail"
	}

	s.updateContractInfo(app, info)

	if native != nil {
		s.lock.Lock()
		s.succ[app.Name] = native
		s.lock.Unlock()

		app.native = native
		app.Printf("[AotService] NewNative ok: app:%s", app.Name)
	}
	return err
}

func (s *AotService) doCompile(app *APP) (*ContractInfo, error) {
	info := ContractInfo{
		Type: "wasm",
		Err:  "",
	}

	// exec.SetCGenLogger(app.logger) // for debug
	ctx := exec.NewCGenContext(app.VM, s.keepCSource)

	// @Todo: for debug
	if app.Name == "0x00000000000000000000466f756e646174696f6e" {
		ctx.EnableComment(true)
	}

	code, err := ctx.Generate()
	if err != nil {
		info.Err = "Generate C Code Fail"
		return &info, err
	}

	file, err := ctx.Compile(code, s.path, app.Name)
	if err != nil {
		info.Err = "Compile C Code Fail"
		return &info, err
	}

	info.Path = file
	info.MD5 = md5.Sum(code)
	app.Printf("[AotService] doCompile ok: app:%s", app.Name)
	return &info, nil
}

var (
	contractInfoPrefix = []byte("cfso:")
)

const (
	contractInfoPrefixLen = 5
)

func (s *AotService) updateContractInfo(app *APP, info *ContractInfo) {
	if info.Err != "" {
		s.black[app.Name] = struct{}{}
	}

	data, err := json.Marshal(info)
	if err != nil {
		app.Printf("[AotService] json.Marshal ContractInfo fail: %s", err)
		return
	}

	key := make([]byte, types.AddressLength+contractInfoPrefixLen)
	copy(key[:contractInfoPrefixLen], contractInfoPrefix)
	copy(key[contractInfoPrefixLen:], types.HexToAddress(app.Name).Bytes())

	stateDB := app.Eng.State
	stateDB.SetContractInfo(key, data)
}

func (s *AotService) getContractInfo(app *APP) *ContractInfo {
	key := make([]byte, types.AddressLength+contractInfoPrefixLen)
	copy(key[:contractInfoPrefixLen], contractInfoPrefix)
	copy(key[contractInfoPrefixLen:], types.HexToAddress(app.Name).Bytes())

	stateDB := app.Eng.State
	data := stateDB.GetContractInfo(key)
	if len(data) == 0 {
		return nil
	}

	var info ContractInfo
	if err := json.Unmarshal(data, &info); err != nil {
		app.Printf("[AotService] json.Unmarshal ContractInfo fail: app:%s, err:%s", app.Name, err)
		return nil
	}
	return &info
}

func init() {
	path := os.Getenv(TCVM_AOTS_ROOT)
	if path == "" {
		path = "/tmp/aots"
	}
	if err := os.MkdirAll(path, 0775); err != nil {
		fmt.Printf("%s = %s, MkdirAll fail: %s\n", TCVM_AOTS_ROOT, path, err)
	} else {
		fmt.Printf("%s = %s, MkdirAll ok\n", TCVM_AOTS_ROOT, path)
	}

	keepSource := true
	if os.Getenv(TCVM_AOTS_KEEP_CSOURCE) == "0" {
		keepSource = false
	}

	aots = NewAotService(path, keepSource)
	go aots.loop()
}
