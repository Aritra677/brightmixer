package main

import (
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
)

// ANSI Escape Sequences for TUI control
const (
	altScreenOn  = "\033[?1049h"
	altScreenOff = "\033[?1049l"
	cursorHide   = "\033[?25l"
	cursorShow   = "\033[?25h"
	clearScreen  = "\033[2J\033[H"
	colorCyan    = "\033[36m"
	colorYellow  = "\033[33m"
	colorGray    = "\033[90m"
	colorReset   = "\033[0m"
)

// setRawMode toggles canonical line-buffering and echo via stty
func setRawMode(enable bool) {
	var cmd *exec.Cmd
	if enable {
		cmd = exec.Command("stty", "raw", "-echo")
	} else {
		cmd = exec.Command("stty", "-raw", "echo")
	}
	cmd.Stdin = os.Stdin
	_ = cmd.Run()
}

// getBrightness queries brightnessctl for current and max hardware values
func getBrightness() (int, int) {
	curOut, err := exec.Command("brightnessctl", "g").Output()
	if err != nil {
		return 0, 100
	}
	maxOut, err := exec.Command("brightnessctl", "m").Output()
	if err != nil {
		return 0, 100
	}
	cur, _ := strconv.Atoi(strings.TrimSpace(string(curOut)))
	max, _ := strconv.Atoi(strings.TrimSpace(string(maxOut)))
	if max == 0 {
		return 0, 100
	}
	return cur, max
}

// setBrightness clamps percentage between 5% and 100% and applies it
func setBrightness(pct int) {
	if pct < 5 {
		pct = 5
	}
	if pct > 100 {
		pct = 100
	}
	_ = exec.Command("brightnessctl", "set", fmt.Sprintf("%d%%", pct)).Run()
}

// render draws the alsamixer-style vertical bar onto the alternate screen
func render(pct int) {
	const barHeight = 20
	filled := (pct * barHeight) / 100

	var sb strings.Builder
	sb.WriteString(clearScreen)
	sb.WriteString(fmt.Sprintf("  %sBrightMixer%s (brightnessctl)\r\n", colorCyan, colorReset))
	sb.WriteString("  ┌────────┐\r\n")

	for row := barHeight; row >= 1; row-- {
		sb.WriteString("  │ ")
		if row <= filled {
			// Color top 20% yellow like alsamixer peak levels
			if row > 16 {
				sb.WriteString(colorYellow + "██████" + colorReset)
			} else {
				sb.WriteString(colorCyan + "██████" + colorReset)
			}
		} else {
			sb.WriteString(colorGray + "░░░░░░" + colorReset)
		}
		sb.WriteString(" │\r\n")
	}

	sb.WriteString("  └────────┘\r\n")
	sb.WriteString(fmt.Sprintf("    %3d%%\r\n\r\n", pct))
	sb.WriteString("  [↑/k] +5%   [↓/j] -5%   [1-9] Jump   [q] Quit\r\n")

	fmt.Print(sb.String())
}

func cleanup() {
	setRawMode(false)
	fmt.Print(cursorShow + altScreenOff)
}

func main() {
	// Always restore terminal state on normal exit or Ctrl+C / SIGTERM
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigChan
		cleanup()
		os.Exit(0)
	}()

	fmt.Print(altScreenOn + cursorHide)
	setRawMode(true)
	defer cleanup()

	cur, max := getBrightness()
	pct := (cur * 100) / max
	render(pct)

	buf := make([]byte, 3)
	for {
		n, err := os.Stdin.Read(buf)
		if err != nil || n == 0 {
			break
		}

		// Single-byte keys
		if n == 1 {
			switch buf[0] {
			case 'q', 'Q', 3: // 'q' or Ctrl+C (ASCII 3)
				return
			case 'k', 'K', '+', '=':
				pct += 5
			case 'j', 'J', '-':
				pct -= 5
			case '1', '2', '3', '4', '5', '6', '7', '8', '9':
				pct = int(buf[0]-'0') * 10
			case '0':
				pct = 100
			}
		} else if n == 3 && buf[0] == 27 && buf[1] == '[' {
			// 3-byte ANSI Arrow Key Escape Sequences: ESC [ A (Up), ESC [ B (Down)
			switch buf[2] {
			case 'A':
				pct += 5
			case 'B':
				pct -= 5
			}
		}

		if pct < 5 {
			pct = 5
		} else if pct > 100 {
			pct = 100
		}

		setBrightness(pct)
		render(pct)
	}
}
