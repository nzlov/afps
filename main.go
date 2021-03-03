package main

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"
)

var (
	TOUCHING = int64(0)
	FPS      = "60"
	LOCK     *sync.Mutex
	M        map[string]PF
	CFGTIME  = int64(0)
	ACTIVITY = "*"
	CPF      PF
)

const (
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
	fmt.Println("loadConfig...")
	initConfig()

	f, err := os.Open(CFGPATH)
	if err != nil {
		return err
	}
	fs, err := f.Stat()
	if err != nil {
		return err
	}
	if fs.ModTime().Unix() == CFGTIME {
		fmt.Println("loadConfig not changed")
		return nil
	}
	CFGTIME = fs.ModTime().Unix()
	fmt.Println("loadConfiging")
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
		fmt.Println("loadConfig:", string(l))
		M[ls[0]] = PF{
			idle:     ls[1],
			touching: ls[2],
		}

	}
	fmt.Println("loadConfig ok")
	return nil
}

func main() {
	loadConfig()
	changeActivity("*")

	LOCK = &sync.Mutex{}
	c := exec.Command("getevent")
	c.Stderr = &W{}
	c.Stdout = &W{}
	start()
	fmt.Println(c.Run())
}

func start() {
	go func() {
		t := time.NewTicker(time.Second)
		for n := range t.C {
			loadConfig()
			changeActivity(getActivity())
			if n.Unix()-TOUCHING > 1 {
				upfps(CPF.idle)
			}
		}
	}()
}

type PF struct {
	idle     string
	touching string
}

type W struct {
}

func (w *W) Write(p []byte) (n int, err error) {
	TOUCHING = time.Now().Unix()
	upfps(CPF.touching)
	return len(p), nil
}

func upfps(i string) {
	LOCK.Lock()
	defer LOCK.Unlock()
	if FPS == i {
		return
	}
	fmt.Println("upfps:", i)
	FPS = i
	exec.Command("settings", "put", "system", "min_refresh_rate", FPS).Output()
}

func changeActivity(a string) {
	if a == ACTIVITY {
		return
	}
	fmt.Println("changeActivity:", a)
	ACTIVITY = a
	CPF = getPF(a)
}

func getPF(n string) PF {
	if n == "" {
		n = "*"
	}
	if v, ok := M[n]; ok {
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
