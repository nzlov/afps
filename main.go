package main

import (
	"bufio"
	"bytes"
	"io"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
)

var (
	TOUCHING = int64(0)
	FPS      = "60"
	SLOCK    *sync.Mutex
	FLOCK    *sync.Mutex
	M        map[string]PF
	CFGTIME  = int64(0)
	ACTIVITY = "*"
	CPF      PF
	w        *W
	c        *exec.Cmd
	t        *time.Ticker
	Running  bool
)

const (
	LEDPATH = "/sys/class/leds/lcd-backlight/brightness"
	CFGPATH = "/sdcard/afps_nzlov.conf"
)

func initConfig() {
	if _, err := os.Stat(CFGPATH); err != nil {
		os.WriteFile(CFGPATH, []byte(`# 欢迎使用
# 包名 空闲FPS 触摸FPS
# tv.danmaku.bili 60 120
# * 60 60
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

	r := bufio.NewReader(f)
	for {
		l, _, err := r.ReadLine()
		if err != nil {
			if err == io.EOF {
				break
			}
			return err
		}
		ls := strings.Split(strings.TrimSpace(string(l)), " ")
		if len(ls) != 3 {
			continue
		}
		log("loadConfig:", string(l))
		M[ls[0]] = PF{
			idle:     ls[1],
			touching: ls[2],
		}

	}
	log("loadConfig ok")
	return nil
}

func main() {

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
			if n.Unix()-TOUCHING > 1 {
				upfps(CPF.idle)
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
}

type PF struct {
	idle     string
	touching string
}

type W struct {
}

func (w *W) Write(p []byte) (n int, err error) {
	TOUCHING = time.Now().Unix()
	log(string(p))
	upfps(CPF.touching)
	return len(p), nil
}

func upfps(i string) {
	FLOCK.Lock()
	defer FLOCK.Unlock()
	if FPS == i {
		return
	}
	log("upfps:", i)
	FPS = i
	exec.Command("settings", "put", "system", "min_refresh_rate", FPS).Output()
}

func changeActivity(a string) {
	if a == ACTIVITY {
		return
	}
	log("changeActivity:", a)
	ACTIVITY = a
	CPF = getPF(a)
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
