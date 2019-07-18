package main

import (
	"bufio"
	"bytes"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"sort"
	"strconv"
	"sync"
	// "time"
)

type Info struct {
	Pid  int
	Size int64
	Comm string
}

var (
	nullBytes  = []byte{0x0}
	emptyBytes = []byte(" ")
	swapPrefix = []byte("Swap:")

	procString    = "/proc/"
	cmdlineString = "/cmdline"
	smapsString   = "/smaps"
)

func main() {
	slist := GetInfos()
	sort.Slice(slist, func(i, j int) bool {
		return slist[i].Size < slist[j].Size
	})

	fmt.Printf("%5s %9s %s\n", "PID", "SWAP", "COMMAND")
	var total int64
	for _, v := range slist {
		fmt.Printf("%5d %9s %s\n", v.Pid, FormatSize(v.Size), v.Comm)
		total += v.Size
	}
	fmt.Printf("Total: %8s\n", FormatSize(total))
}

func GetInfos() (list []Info) {
	f, _ := os.Open("/proc")
	defer f.Close()
	names, err := f.Readdirnames(0)
	if err != nil {
		log.Fatalf("read /proc: %v", err)
	}
	length := len(names)

	list = make([]Info, length)
	infoCh := make(chan *Info, length)
	wg := new(sync.WaitGroup)
	wg2 := new(sync.WaitGroup)

	wg2.Add(1)
	go func(info_ch chan *Info, list *[]Info) {
		defer wg2.Done()

		idx := 0
		for tmp := range info_ch {
			(*list)[idx] = *tmp
		}
	}(infoCh, &list)

	wg.Add(length)
	for _, name := range names {
		go GetInfo(name, infoCh, wg)
	}

	wg.Wait()
	close(infoCh)
	wg2.Wait()
	return
}

func GetInfo(name string, infoCh chan<- *Info, wg *sync.WaitGroup) {
	defer wg.Done()

	pid, err := strconv.Atoi(name)
	if err != nil {
		return
	}

	info := new(Info)
	info.Pid = pid

	var bs []byte
	bs, err = ioutil.ReadFile(fmt.Sprintf("/proc/%d/cmdline", pid))
	if err != nil {
		return
	}
	if bytes.HasSuffix(bs, nullBytes) {
		bs = bs[:len(bs)-1]
	}
	info.Comm = string(bytes.Replace(bs, nullBytes, emptyBytes, -1))

	bs, err = ioutil.ReadFile(fmt.Sprintf("/proc/%d/smaps", pid))
	if err != nil {
		return
	}

	var total int64
	var b []byte

	r := bufio.NewScanner(bytes.NewReader(bs))
	size := int64(0)
	for r.Scan() {
		b = r.Bytes()
		if !bytes.HasPrefix(b, swapPrefix) {
			continue
		}

		size = 0
		for _, v := range b {
			if v < '0' || v > '9' {
				continue
			}
			size = size*10 + int64(v-'0')
		}

		total += size
	}

	info.Size = total * 1024
	infoCh <- info
	return
}

var units = []string{"", "K", "M", "G", "T"}

func FormatSize(s int64) string {
	unit := 0
	f := float64(s)
	for unit < len(units) && f > 1100.0 {
		f /= 1024.0
		unit++
	}
	if unit == 0 {
		return fmt.Sprintf("%dB", int64(f))
	} else {
		return fmt.Sprintf("%.1f%siB", f, units[unit])
	}
}
