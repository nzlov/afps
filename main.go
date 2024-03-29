package main

import (
	"bufio"
	"bytes"
	"context"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
)

var (
	TOUCHTIME time.Time
	TOUCHING  = false
	FPS       = "60"
	SLOCK     *sync.Mutex
	FLOCK     *sync.Mutex
	M         map[string]PF
	CFGTIME   = int64(0)
	ACTIVITY  = "*"
	CPF       PF
	w         *W
	c         *exec.Cmd
	tt        *time.Ticker
	pt        *time.Ticker
	Running   bool
	INTERVAL  = time.Second
	CINTERVAL = time.Second
	MODE      = "def"
	DONE      chan struct{}
)

const (
	LEDPATH = "/sys/class/leds/lcd-backlight/brightness"
	CFGPATH = "/sdcard/afps_nzlov.conf"
)

func initConfig() {
	if _, err := os.Stat(CFGPATH); err != nil {
		f, err := os.Create(CFGPATH)
		if err != nil {
			log("initConfig,createConfig", err)
			time.Sleep(time.Second)
			initConfig()
			return
		}
		defer f.Close()
		if _, err = f.Write([]byte(`# 欢迎使用
# 具体用法访问 https://github.com/nzlov/afps 或者 https://gitee.com/nzlov/afps
# 包名 空闲FPS 触摸FPS 延迟(毫秒)
# tv.danmaku.bili 60 120
# tv.danmaku.bili/.MainActivity2 60 120 200
# * 60 60 1000
# 没有添加延迟的条目使用*延迟配置，如果*也不存在，默认使用1s

# 导入线上配置
# @import https://gitee.com/nzlov/afps/raw/main/global.conf

# 设置模式 def 默认，close 关闭 ci 启用自定义延迟(增加耗电),默认模式下依然会读取*的延迟
@mode def

* 60 120 1000
`)); err != nil {
			log("initConfig", err)
			time.Sleep(time.Second)
			initConfig()
		}
	}
}

var ll = &sync.Mutex{}

func loadConfig() error {
	ll.Lock()
	defer ll.Unlock()
	log("loadConfig...")
	initConfig()

	f, err := os.Open(CFGPATH)
	if err != nil {
		return err
	}
	defer f.Close()
	fs, err := f.Stat()
	if err != nil {
		return err
	}
	if fs.ModTime().Unix() == CFGTIME {
		log("loadConfig not changed")
		return nil
	}
	CFGTIME = fs.ModTime().Unix()
	log("loadConfiging")
	M = map[string]PF{}

	if err := loadconfig(f); err != nil {
		return err
	}

	if _, ok := M["*"]; !ok {
		M["*"] = PF{"60", "60", 1000}
	}
	log("loadConfig ok")
	return nil
}

func loadconfig(r io.Reader) error {
	b := bufio.NewReader(r)
	for {
		l, _, err := b.ReadLine()
		if err != nil {
			if err == io.EOF {
				break
			}
			return err
		}
		ls := strings.TrimSpace(string(l))

		if strings.HasPrefix(ls, "#") {
			continue
		}

		if strings.HasPrefix(ls, "@import ") {
			log("loadConfig import:", string(ls[8:]))
			g := get(string(ls[8:]))
			loadconfig(bytes.NewBuffer(g))
			continue
		}

		if strings.HasPrefix(ls, "@mode ") {
			log("loadConfig mode:", string(ls[6:]))
			switch strings.TrimSpace(string(ls[6:])) {
			case "ci":
				MODE = "ci"
			case "close":
				MODE = "close"
			default:
				MODE = "def"
			}
			continue
		}

		lss := strings.Split(strings.TrimSpace(ls), " ")
		if len(lss) < 3 {
			continue
		}
		log("loadConfig:", ls)
		pf := PF{
			idle:     lss[1],
			touching: lss[2],
		}

		if len(lss) == 4 {
			v, _ := strconv.ParseInt(lss[3], 10, 64)
			pf.interval = time.Duration(v) * time.Millisecond
		}

		if lss[0] == "*" {
			if pf.interval < 1 {
				pf.interval = time.Second
			}
			INTERVAL = pf.interval
			CINTERVAL = INTERVAL
		}

		M[lss[0]] = pf
	}

	return nil
}

func get(host string) []byte {
	resp, err := http.Get(host)
	if err != nil {
		log("get host:", host, err)
		return []byte{}
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(resp.Body)
	log("get host:", host, ":", string(data))
	return data
}

func main() {

	var (
		dnsResolverIP        = "114.114.114.114:53" // Google DNS resolver.
		dnsResolverProto     = "udp"                // Protocol to use for the DNS resolver
		dnsResolverTimeoutMs = 5000                 // Timeout (ms) for the DNS resolver (optional)
	)

	dialer := &net.Dialer{
		Resolver: &net.Resolver{
			PreferGo: true,
			Dial: func(ctx context.Context, network, address string) (net.Conn, error) {
				d := net.Dialer{
					Timeout: time.Duration(dnsResolverTimeoutMs) * time.Millisecond,
				}
				return d.DialContext(ctx, dnsResolverProto, dnsResolverIP)
			},
		},
	}

	dialContext := func(ctx context.Context, network, addr string) (net.Conn, error) {
		return dialer.DialContext(ctx, network, addr)
	}

	http.DefaultTransport.(*http.Transport).DialContext = dialContext

	loadConfig()
	changeActivity("*")

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		log(err)
	}
	defer watcher.Close()

	go func() {
		defer func() {
			log("watcher exit")
		}()
		for {
			log("watcher for")
			select {
			case event, ok := <-watcher.Events:
				if !ok {
					return
				}
				log("event:", event)
				if event.Op&fsnotify.Write == fsnotify.Write {
					switch event.Name {
					case CFGPATH:
						stop()
						loadConfig()
						start()
					case LEDPATH:
						data, _ := os.ReadFile(LEDPATH)
						log("rled:", string(data[0]))
						if "0" == string(data[0]) {
							stop()
						} else if !Running {
							start()
						}
					}
				}
			case err, ok := <-watcher.Errors:
				if !ok {
					return
				}
				log("error:", err)
			}
		}
	}()

	err = watcher.Add(LEDPATH)
	if err != nil {
		log(err)
	}
	err = watcher.Add(CFGPATH)
	if err != nil {
		log(err)
	}

	SLOCK = &sync.Mutex{}
	FLOCK = &sync.Mutex{}
	w = &W{}

	tt = time.NewTicker(CINTERVAL)
	tt.Stop()
	pt = time.NewTicker(time.Second)
	pt.Stop()

	start()

	select {}
}

func start() {
	SLOCK.Lock()
	if Running {
		SLOCK.Unlock()
		return
	}
	if MODE == "close" {
		SLOCK.Unlock()
		return
	}
	Running = true
	SLOCK.Unlock()

	DONE = make(chan struct{})

	log("start", MODE, CINTERVAL)
	switch MODE {
	case "ci":
		tt.Reset(CINTERVAL)
		pt.Reset(time.Second)
		go func() {
			defer log("stop tt")
			for {
				select {
				case n := <-tt.C:
					if n.Sub(TOUCHTIME) > CINTERVAL {
						upfps(CPF.idle)
						TOUCHING = false
					}
				case <-DONE:
					return
				}
			}
		}()
		go func() {
			defer log("stop pt")
			for {
				select {
				case <-pt.C:
					changeActivity(getActivity())
				case <-DONE:
					return

				}
			}
		}()
	default:
		tt.Reset(INTERVAL)
		go func() {
			defer log("stop tt")
			for {
				select {
				case n := <-tt.C:
					if n.Sub(TOUCHTIME) > CINTERVAL {
						upfps(CPF.idle)
						TOUCHING = false
					}
					changeActivity(getActivity())
				case <-DONE:
					return
				}
			}
		}()
	}

	c = exec.Command("getevent")
	c.Stderr = w
	c.Stdout = w
	c.Start()
}

func getInterval() time.Duration {
	if CPF.interval < time.Millisecond {
		return INTERVAL
	}
	return CPF.interval
}

func stop() {
	if !Running {
		return
	}
	SLOCK.Lock()
	if !Running {
		SLOCK.Unlock()
		return
	}
	Running = false
	SLOCK.Unlock()

	close(DONE)

	log("stop")
	if tt != nil {
		tt.Stop()
	}
	if pt != nil {
		pt.Stop()
	}
	if c != nil {
		c.Process.Kill()
	}
	upfps(CPF.idle)
	TOUCHING = false
}

type PF struct {
	idle     string
	touching string
	interval time.Duration
}

type W struct {
}

func (w *W) Write(p []byte) (n int, err error) {
	TOUCHTIME = time.Now()
	if TOUCHING {
		return len(p), nil
	}
	upfps(CPF.touching)
	return len(p), nil
}

func upfps(i string) {
	FLOCK.Lock()
	if FPS == i {
		FLOCK.Unlock()
		return
	}
	FPS = i
	FLOCK.Unlock()

	TOUCHING = true
	log("upfps:", i)
	exec.Command("settings", "put", "system", "min_refresh_rate", FPS).Output()
	exec.Command("settings", "put", "system", "peak_refresh_rate", FPS).Output()
}

func changeActivity(a string) {
	if a == ACTIVITY {
		return
	}
	log("changeActivity:", a)
	ACTIVITY = a
	CPF = getPF(a)
	if TOUCHING {
		upfps(CPF.touching)
	}
	CINTERVAL = getInterval()
	tt.Reset(CINTERVAL)
}

func getPF(n string) PF {
	if v, ok := M[n]; ok {
		return v
	}

	ns := strings.Index(n, "/")

	if ns > -1 {
		if v, ok := M[n[:ns]]; ok {
			return v
		}
	}

	if v, ok := M["*"]; ok {
		return v
	}

	return PF{"60", "120", 1000}
}

func getActivity() string {
	data, err := exec.Command("dumpsys", "activity", "activities").Output()
	if err != nil {
		panic(err)
	}
	br := bufio.NewReader(bytes.NewReader(data))
	for {
		l, _, err := br.ReadLine()
		if err != nil {
			if err == io.EOF {
				break
			}
			break
		}
		s := strings.Index(strings.TrimSpace(string(l)), "ActivityRecord{")
		if s > -1 {
			ls := strings.Split(string(l[s+15:]), " ")
			if len(ls) > 2 {
				return ls[2]
			}
		}
	}
	return ""
}
