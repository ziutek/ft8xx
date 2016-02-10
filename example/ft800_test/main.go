package main

import (
	"bufio"
	"fmt"

	"github.com/ziutek/bitbang/spi"
	"github.com/ziutek/ft8xx"
	"github.com/ziutek/ftdi"
)

// Connections (FT232RL -- FT800):
// TxD (DBUS0) -- PDn
// RxD (DBUS1) -- INTn
// RTS (DBUS2) -- CSn
// CTS (DBUS3) -- MOSI
// DTR (DBUS4) -- MISO
// DSR (DBUS5) -- SCLK
// GND         -- GND
// 5V          -- VCC
const (
	PDn  = 0x01
	INTn = 0x02
	CSn  = 0x04
	MOSI = 0x08
	MISO = 0x10
	SCLK = 0x20
)

func main() {
	udevs, err := ftdi.FindAll(0x0403, 0x6001)
	checkErr(err)
	if len(udevs) == 0 {
		die("Can not find FT232 device.")
	}
	fmt.Printf("Found %d FT232 devices:\n", len(udevs))
	for i, udev := range udevs {
		fmt.Printf("%d: %s\n", i, udev.Serial)
	}

	fmt.Println("Use 0.")

	ft232, err := ftdi.OpenUSBDev(udevs[0], ftdi.ChannelAny)
	checkErr(err)
	checkErr(ft232.SetBitmode(SCLK|MOSI|PDn|CSn, ftdi.ModeSyncBB))

	checkErr(ft232.SetBaudrate(512 * 1024 / 16))
	const cs = 4096
	checkErr(ft232.SetReadChunkSize(cs))
	checkErr(ft232.SetWriteChunkSize(cs))
	checkErr(ft232.SetLatencyTimer(2))
	checkErr(ft232.PurgeBuffers())

	ma := spi.NewMaster(
		&spiDrv{r: ft232, w: bufio.NewWriterSize(ft232, cs)},
		SCLK, MOSI, MISO,
	)
	cfg := spi.Config{
		Mode:     spi.MSBF | spi.CPOL0 | spi.CPHA0,
		FrameLen: 1,
		Delay:    0,
	}
	ma.Configure(cfg)

	prePost := []byte{CSn | PDn}
	ma.SetBase(PDn)
	ma.SetPrePost(prePost, prePost)

	eve := ft8xx.EVE{M: ma}

	// The following code is inexact translation of
	// "FTDI Simplified ARM Application" (see AN_312).

	fmt.Print("Reset... ")

	checkErr(ft232.WriteByte(PDn | CSn))
	checkIErr(ft232.ReadByte())
	delay(20)
	checkErr(ft232.WriteByte(CSn))
	checkIErr(ft232.ReadByte())
	delay(20)
	checkErr(ft232.WriteByte(PDn | CSn))
	checkIErr(ft232.ReadByte())
	delay(20)

	fmt.Println("OK.")

	fmt.Print("Init... ")

	eve.HostCmd(ft8xx.FT800_ACTIVE)
	delay(5)
	eve.HostCmd(ft8xx.FT800_CLKEXT)
	delay(5)
	eve.HostCmd(ft8xx.FT800_CLK48M)
	delay(5)
	checkErr(eve.Err)

	fmt.Println("OK.")

	fmt.Print("Checkg FT8xx device type... ")

	if eve.Read8(ft8xx.REG_ID) != 0x7c {
		checkErr(eve.Err)
		die("Unknown chip ID")
	}

	fmt.Println(" FT800.")

	fmt.Print("Configure WQVGA (480x272) display... ")

	eve.Write8(ft8xx.REG_PCLK, 0)
	eve.Write8(ft8xx.REG_PWM_DUTY, 0)
	checkErr(eve.Err)

	const (
		lcdWidth   = 480 // Active width of LCD display
		lcdHeight  = 272 // Active height of LCD display
		lcdHcycle  = 548 // Total number of clocks per line
		lcdHoffset = 43  // Start of active line
		lcdHsync0  = 0   // Start of horizontal sync pulse
		lcdHsync1  = 41  // End of horizontal sync pulse
		lcdVcycle  = 292 // Total number of lines per screen
		lcdVoffset = 12  // Start of active screen
		lcdVsync0  = 0   // Start of vertical sync pulse
		lcdVsync1  = 10  // End of vertical sync pulse
		lcdPclk    = 5   // Pixel Clock
		lcdSwizzle = 0   // Define RGB output pins
		lcdPclkpol = 1   // Define active edge of PCLK
	)

	eve.Write16(ft8xx.REG_HSIZE, lcdWidth)
	eve.Write16(ft8xx.REG_HCYCLE, lcdHcycle)
	eve.Write16(ft8xx.REG_HOFFSET, lcdHoffset)
	eve.Write16(ft8xx.REG_HSYNC0, lcdHsync0)
	eve.Write16(ft8xx.REG_HSYNC1, lcdHsync1)
	eve.Write16(ft8xx.REG_VSIZE, lcdHeight)
	eve.Write16(ft8xx.REG_VCYCLE, lcdVcycle)
	eve.Write16(ft8xx.REG_VOFFSET, lcdVoffset)
	eve.Write16(ft8xx.REG_VSYNC0, lcdVsync0)
	eve.Write16(ft8xx.REG_VSYNC1, lcdVsync1)
	eve.Write8(ft8xx.REG_SWIZZLE, lcdSwizzle)
	eve.Write8(ft8xx.REG_PCLK_POL, lcdPclkpol)
	checkErr(eve.Err)

	fmt.Println("OK.")

	// TODO: Configure Touch and Audio.

	fmt.Print("Write initial display list and enable display... ")

	var offset int
	offset = eve.WriteDL(offset, ft8xx.DL_CLEAR_RGB)
	offset = eve.WriteDL(offset, ft8xx.DL_CLEAR|ft8xx.CLR_COL|ft8xx.CLR_STN|ft8xx.CLR_TAG)
	offset = eve.WriteDL(offset, ft8xx.DL_DISPLAY)
	eve.Write32(ft8xx.REG_DLSWAP, ft8xx.DLSWAP_FRAME)
	checkErr(eve.Err)

	gpio := eve.Read8(ft8xx.REG_GPIO)
	eve.Write8(ft8xx.REG_GPIO, gpio|0x80)
	eve.Write8(ft8xx.REG_PCLK, lcdPclk)
	for duty := 0; duty <= 100; duty++ {
		eve.Write8(ft8xx.REG_PWM_DUTY, duty)
		delay(5)
	}
	checkErr(eve.Err)

	fmt.Println("OK.")

	const (
		white   = 0xFFFFFF
		blue    = 0x0000FF
		yellow0 = 0x888800
		yellow1 = 0xffff33
		black   = 0x000000
	)

	fifoWait := func() {
		for {

			cmdBufferRd := eve.Read16(ft8xx.REG_CMD_READ)
			cmdBufferWr := eve.Read16(ft8xx.REG_CMD_WRITE)
			checkErr(eve.Err)
			if cmdBufferRd == cmdBufferWr {
				break
			}
			//delay(1)
		}
	}

	fmt.Print("Touch panel calibration... ")

	offset = 0
	offset = eve.WriteCmd(offset, ft8xx.CMD_DLSTART)
	offset = eve.WriteCmd(offset, ft8xx.DL_CLEAR_RGB|black)
	offset = eve.WriteCmd(offset, ft8xx.DL_CLEAR|ft8xx.CLR_COL|ft8xx.CLR_STN|ft8xx.CLR_TAG)
	offset = eve.WriteCmd(offset, ft8xx.DL_COLOR_RGB|white)
	offset = eve.WriteText(offset, lcdWidth/2, lcdHeight/2, 31, ft8xx.OPT_CENTER, "Touch panel calibration")
	offset = eve.WriteCmd(offset, ft8xx.CMD_BGCOLOR)
	offset = eve.WriteCmd(offset, ft8xx.DL_COLOR_RGB|yellow0)
	offset = eve.WriteCmd(offset, ft8xx.CMD_FGCOLOR)
	offset = eve.WriteCmd(offset, ft8xx.DL_COLOR_RGB|yellow1)
	offset = eve.WriteCmd(offset, ft8xx.CMD_CALIBRATE)
	offset = eve.WriteCmd(offset, 0)
	eve.Write16(ft8xx.REG_CMD_WRITE, offset)
	checkErr(eve.Err)

	fifoWait()

	status := eve.Read32(ft8xx.RAM_CMD + offset - 4)
	checkErr(eve.Err)

	if status != 0 {
		var tr [6]int
		tr[0] = eve.Read32(ft8xx.REG_TOUCH_TRANSFORM_A)
		tr[1] = eve.Read32(ft8xx.REG_TOUCH_TRANSFORM_B)
		tr[2] = eve.Read32(ft8xx.REG_TOUCH_TRANSFORM_C)
		tr[3] = eve.Read32(ft8xx.REG_TOUCH_TRANSFORM_D)
		tr[4] = eve.Read32(ft8xx.REG_TOUCH_TRANSFORM_E)
		tr[5] = eve.Read32(ft8xx.REG_TOUCH_TRANSFORM_F)
		fmt.Println("OK.")
		for _, v := range tr {
			fmt.Printf("  % 7.2f (0x%08x)\n", float64(int32(v))/65536, v)
		}
	} else {
		fmt.Println("failed!")
	}

	fmt.Println("Animation...")

	x := 96 * 16
	y := 136 * 16
	dx := 31
	dy := 47
	r := 24 * 16

	for {
		offset = eve.WriteCmd(offset, ft8xx.CMD_DLSTART)
		offset = eve.WriteCmd(offset, ft8xx.DL_CLEAR_RGB|black)
		offset = eve.WriteCmd(offset, ft8xx.DL_CLEAR|ft8xx.CLR_COL|ft8xx.CLR_STN|ft8xx.CLR_TAG)

		offset = eve.WriteCmd(offset, ft8xx.DL_COLOR_RGB|white)
		offset = eve.WriteText(offset, lcdWidth/2, lcdHeight/2, 31, ft8xx.OPT_CENTER, "FT800 test application")
		offset = eve.WriteCmd(offset, ft8xx.DL_COLOR_RGB|blue)
		offset = eve.WriteCmd(offset, ft8xx.DL_POINT_SIZE|r)
		offset = eve.WriteCmd(offset, ft8xx.DL_BEGIN|ft8xx.FTPOINTS)
		offset = eve.WriteCmd(offset, ft8xx.DL_VERTEX2F|(x<<15|y))
		offset = eve.WriteCmd(offset, ft8xx.DL_END)
		offset = eve.WriteCmd(offset, ft8xx.DL_DISPLAY)
		offset = eve.WriteCmd(offset, ft8xx.CMD_SWAP)
		eve.Write16(ft8xx.REG_CMD_WRITE, offset)
		checkErr(eve.Err)

		x += dx
		if x+r >= lcdWidth*16 || x-r < 0 {
			dx = -dx
		}
		y += dy
		if y+r >= lcdHeight*16 || y-r < 0 {
			dy = -dy
		}

		fifoWait()
	}
}
