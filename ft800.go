package ft8xx

type HostCmd byte

const (
	FT800_ACTIVE  HostCmd = 0x00 // Initializes FT800
	FT800_STANDBY HostCmd = 0x41 // Place FT800 in Standby (clk running)
	FT800_SLEEP   HostCmd = 0x42 // Place FT800 in Sleep (clk off)
	FT800_PWRDOWN HostCmd = 0x50 // Place FT800 in Power Down (core off)
	FT800_CLKEXT  HostCmd = 0x44 // Select external clock source
	FT800_CLK48M  HostCmd = 0x62 // Select 48MHz PLL
	FT800_CLK36M  HostCmd = 0x61 // Select 36MHz PLL
	FT800_CORERST HostCmd = 0x68 // Reset core - all registers default
)
