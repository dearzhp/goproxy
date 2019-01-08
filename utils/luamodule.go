package utils

import (
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"

	"github.com/yuin/gopher-lua"
)

type lStatePool struct {
	saved chan *lua.LState
	dir   string
	entry string
}

func (pl *lStatePool) Get() *lua.LState {
	return <-pl.saved
}

func (pl *lStatePool) New() (*lua.LState, error) {
	return loadLua(pl.dir, pl.entry)
}

func (pl *lStatePool) Put(L *lua.LState) {
	pl.saved <- L
}

func (pl *lStatePool) Shutdown() {
	L := <-pl.saved
	L.Close()
	close(pl.saved)
}

func LuaPool(luaDir, entry string) *lStatePool {
	luaPool := &lStatePool{
		saved: make(chan *lua.LState, 1),
		dir:   luaDir,
		entry: entry,
	}
	L, _ := luaPool.New()
	luaPool.saved <- L
	return luaPool
}

func isLua(f string) (bool, string) {
	if !PathExists(f) {
		return false, ""
	}
	if info, err := os.Stat(f); err == nil && info.IsDir() {
		entry := filepath.Join(f, "entry.lua")
		if !PathExists(entry) {
			entry = filepath.Join(f, "entry.luc")
		}
		if PathExists(entry) {
			return true, entry
		}
	}
	return false, ""
}

func loadLua(luaDir, entry string) (*lua.LState, error) {
	os.Setenv("SCRIPT_DIR", luaDir)
	luaPath := filepath.Join(luaDir, "?.lua;")
	luaPath += filepath.Join(luaDir, "?.luc;")
	luaPath += filepath.Join(luaDir, "?/init.lua;")
	luaPath += filepath.Join(luaDir, "?/init.luc;")
	os.Setenv("LUA_PATH", luaPath)
	L := lua.NewState()
	//L.PreloadModule("http", gluahttp.NewHttpModule(&http.Client{}).Loader)
	//L.PreloadModule("url", gluaurl.Loader)
	//gluabit32.Preload(L)
	//gluacrypto.Preload(L)
	L.PreloadModule("ext", Loader)
	return L, L.DoFile(entry)
}

func luaUseProxy(L *lua.LState, domain string) bool {
	if err := L.CallByParam(lua.P{
		Fn:      L.GetGlobal("checkproxy"),
		NRet:    1,
		Protect: true,
	}, lua.LString(domain)); err != nil {
		return false
	}
	defer L.Pop(1)
	return L.ToBool(-1)
}

func Loader(L *lua.LState) int {
	mod := L.SetFuncs(L.NewTable(), exports)
	L.SetField(mod, "name", lua.LString("SPIng golang extension funcions"))
	L.SetField(mod, "script_dir", lua.LString(os.Getenv("SCRIPT_DIR")))
	L.Push(mod)
	return 1
}

var exports = map[string]lua.LGFunction{
	"sync_http_get":  sync_http_get,
	"async_http_get": async_http_get,
}

func sync_http_get(L *lua.LState) int {
	u := L.ToString(1)
	resp, err := http.Get(u)
	if err != nil {
		return 0
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil || len(body) == 0 {
		return 0
	}
	L.Push(lua.LString(string(body)))
	return 1
}

func async_http_get(L *lua.LState) int {
	u := L.ToString(1)
	go func() {
		resp, err := http.Get(u)
		if err != nil {
			resp.Body.Close()
		}
	}()
	return 0
}
