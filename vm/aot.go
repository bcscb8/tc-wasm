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

	"github.com/xunleichain/tc-wasm/mock/log"
	"github.com/xunleichain/tc-wasm/mock/types"
)

// AotService --
type AotService struct {
	path        string
	keepCSource bool
	exit        chan struct{}
	refresh     chan *APP

	black    map[string]struct{}
	succ     map[string]*Native
	onDelete map[string]*Native
	lock     sync.Mutex
	logger   log.Logger
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
		succ:        make(map[string]*Native, 32),
		onDelete:    make(map[string]*Native, 8),
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

// DeleteNative --
func DeleteNative(app *APP) {
	aots.deleteNative(app)
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
	select {
	case s.refresh <- app:
	default:
	}
}

func (s *AotService) getNative(app *APP) *Native {
	s.lock.Lock()
	defer s.lock.Unlock()

	native := s.succ[app.Name]
	return native.clone(app)
}

func (s *AotService) deleteNative(app *APP) {
	s.lock.Lock()

	native := s.succ[app.Name]
	if native != nil {
		s.onDelete[app.Name] = native
		s.succ[app.Name] = nil
		native.remove()
		app.Printf("[AotService] deleteNative begin: app:%s, md5:%s", app.Name, hex.EncodeToString(app.md5[:]))
		s.logger = app.logger
	}

	s.lock.Unlock()
}

func (s *AotService) loop() {
	// idle check timer
	d1 := time.Duration(time.Minute * 5)
	t1 := time.NewTimer(d1)

	// onDelete timer
	d2 := time.Duration(time.Second * 10)
	t2 := time.NewTimer(d2)

	for {
		select {
		case app := <-s.refresh:
			s.lock.Lock()
			if _, ok := s.black[app.Name]; ok {
				s.lock.Unlock()
				continue
			}

			if _, ok := s.onDelete[app.Name]; ok {
				s.lock.Unlock()
				continue
			}

			n := s.succ[app.Name]
			s.lock.Unlock()

			if n == nil {
				app.Printf("[AotService] doCheck: app:%s, md5:%s", app.Name, hex.EncodeToString(app.md5[:]))
				s.doCheck(app)
			}

		case <-t1.C:
			cnt := 0
			now := time.Now()
			target := time.Unix(now.Unix()-3600, 0) // one hour

			s.lock.Lock()
			for name, native := range s.succ {
				if native.t.Before(target) {
					s.succ[name] = nil
					s.onDelete[name] = native
					native.close()

					cnt++
					// fmt.Printf("[AotService] delete native: %s\n", name)
				}
				if cnt >= 3 {
					break
				}
			}
			s.lock.Unlock()

			t1.Reset(d1)

		case <-t2.C:
			s.lock.Lock()
			for name, native := range s.onDelete {
				if native.count() == 0 {
					delete(s.onDelete, name)
					delete(s.succ, name)
					delete(s.black, name)
					s.logger.Info("[AotService] deleteNative done", "app", name)
				}
			}
			s.lock.Unlock()
			t2.Reset(d2)

		case <-s.exit:
			t1.Stop()
			t2.Stop()
			fmt.Printf("[AotService] Exit\n")
			return
		}
	}
}

func (s *AotService) doCheck(app *APP) error {
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
		s.lock.Lock()
		s.black[app.Name] = struct{}{}
		s.lock.Unlock()
		return fmt.Errorf(info.Err)
	}

	stat, err := os.Stat(info.Path)
	if err != nil {
		app.Printf("[AotService] os.Stat %s fail: app:%s, err:%s", info.Path, app.Name, err)
		os.Remove(info.Path)
		return s.doWork(app)
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
		app.Printf("[AotService] NewNative ok: app:%s, md5:%s", app.Name, hex.EncodeToString(app.md5[:]))
		s.lock.Lock()
		s.succ[app.Name] = native
		s.lock.Unlock()
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
	// if app.Name == "0x00000000000000000000466f756e646174696f6e" {
	// 	ctx.EnableComment(true)
	// }

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
	app.Printf("[AotService] doCompile ok: app:%s, md5:%s, so_md5:%s", app.Name, hex.EncodeToString(app.md5[:]), hex.EncodeToString(info.MD5[:]))
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
		s.lock.Lock()
		s.black[app.Name] = struct{}{}
		s.lock.Unlock()
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
