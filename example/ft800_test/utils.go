package main

import (
	"bufio"
	"fmt"
	"os"
	"time"

	"github.com/ziutek/ftdi"
)

func die(a ...interface{}) {
	fmt.Println(a...)
	os.Exit(1)
}

func checkErr(err error) {
	if err == nil {
		return
	}
	die(err)
}

func checkIErr(_ interface{}, err error) {
	checkErr(err)
}

func delay(ms int) {
	time.Sleep(time.Duration(ms) * time.Millisecond)
}

func spitxt(buf []byte) {
	for _, b := range buf {
		txt := []byte(" siocrp")
		if b&SCLK != 0 {
			txt[1] = 'S'
		}
		if b&MISO != 0 {
			txt[2] = 'I'
		}
		if b&MOSI != 0 {
			txt[3] = 'O'
		}
		if b&CSn != 0 {
			txt[4] = 'C'
		}
		if b&INTn != 0 {
			txt[5] = 'R'
		}
		if b&PDn != 0 {
			txt[6] = 'P'
		}
		os.Stdout.Write(txt)
	}
	os.Stdout.Write([]byte{'\n'})
}

type spiDrv struct {
	debug bool
	r     *ftdi.Device
	w     *bufio.Writer
}

func (d *spiDrv) Read(b []byte) (n int, err error) {
	n, err = d.r.Read(b)
	if d.debug {
		os.Stdout.WriteString("read:")
		spitxt(b)
	}
	return
}

func (d *spiDrv) Write(b []byte) (n int, err error) {
	n, err = d.w.Write(b)
	if d.debug {
		os.Stdout.WriteString("write:")
		spitxt(b)
	}
	return
}

func (d *spiDrv) IRQ() (bool, error) {
	b, err := d.r.Pins()
	return b&INTn == 0, err
}

func (d *spiDrv) Flush() error {
	if d.debug {
		fmt.Println("spiflush")
	}
	return d.w.Flush()
}
