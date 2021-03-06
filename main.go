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
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
)

var (
	TOUCHTIME = int64(0)
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
	t         *time.Ticker
	Running   bool
)

const (
	LEDPATH = "/sys/class/leds/lcd-backlight/brightness"
	CFGPATH = "/sdcard/afps_nzlov.conf"
)

func initConfig() {
	if _, err := os.Stat(CFGPATH); err != nil {
		os.WriteFile(CFGPATH, []byte(`# 欢迎使用
# 具体用法访问 https://gitee.com/nzlov/afps
# 包名 空闲FPS 触摸FPS
# tv.danmaku.bili 60 120
# * 60 60

@import https://gitee.com/nzlov/afps/raw/main/global.conf

* 60 120`), 0644)
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

		lss := strings.Split(strings.TrimSpace(ls), " ")
		if len(lss) != 3 {
			continue
		}
		log("loadConfig:", ls)
		M[lss[0]] = PF{
			idle:     lss[1],
			touching: lss[2],
		}

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
						loadConfig()
					case LEDPATH:
						data, _ := os.ReadFile(LEDPATH)
						log("rled:", string(data[0]))
						if "0" == string(data[0]) {
							stop()
						} else {
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

	start()

	select {}
}

func start() {
	SLOCK.Lock()
	defer SLOCK.Unlock()
	if Running {
		return
	}
	Running = true

	log("start")
	t = time.NewTicker(time.Second)
	go func() {
		for n := range t.C {
			changeActivity(getActivity())
			if n.Unix()-TOUCHTIME > 1 {
				upfps(CPF.idle)
				TOUCHING = false
			}
		}
	}()
	c = exec.Command("getevent")
	c.Stderr = w
	c.Stdout = w
	c.Start()
}

func stop() {
	SLOCK.Lock()
	defer SLOCK.Unlock()

	if !Running {
		return
	}
	Running = false

	log("stop")
	if t != nil {
		t.Stop()
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
}

type W struct {
}

func (w *W) Write(p []byte) (n int, err error) {
	TOUCHTIME = time.Now().Unix()
	if TOUCHING {
		log("tend")
		return len(p), nil
	}
	upfps(CPF.touching)
	return len(p), nil
}

func upfps(i string) {
	FLOCK.Lock()
	defer FLOCK.Unlock()
	if FPS == i {
		return
	}
	FPS = i
	TOUCHING = true
	log("upfps:", i)
	exec.Command("settings", "put", "system", "min_refresh_rate", FPS).Output()
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

	return PF{"60", "120"}
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
