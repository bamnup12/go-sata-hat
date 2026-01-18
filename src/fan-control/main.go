package main

import (
    "github.com/baierjan/go-sata-hat/src/common"

    "fmt"
    "log"
    "os"
    "os/signal"
    "strconv"
    "syscall"
    "time"

    "github.com/stianeikeland/go-rpio/v4"
)

var (
    fans          = [2]rpio.Pin{rpio.Pin(common.CPU_FAN_PIN), rpio.Pin(common.DISK_FAN_PIN)}
    current_level uint32
    MIN, _        = strconv.ParseFloat(common.GetEnv("TEMP_MIN", "35.0"), 64)
    MED, _        = strconv.ParseFloat(common.GetEnv("TEMP_MED", "50.0"), 64)
    MAX, _        = strconv.ParseFloat(common.GetEnv("TEMP_MAX", "55.0"), 64)
    BUFFER, _     = strconv.ParseFloat(common.GetEnv("TEMP_BUFFER", "2.0"), 64)
)

func setFan(fan int, level uint32) {
    log.Printf("Setting fan %d to level %d\n", fan, level)
    fans[fan].Mode(rpio.Pwm)
    fans[fan].Freq(100000)
    fans[fan].DutyCycle(level, 4)
}

// calculateLevel determines the appropriate fan level based on temperature
// with hysteresis to prevent rapid switching
func calculateLevel(temp float64, currentLevel uint32) uint32 {
    // Thresholds with hysteresis applied based on current level
    // When going up in speed, use normal thresholds
    // When going down in speed, use threshold - buffer
    thresholds := map[uint32][3]float64{
        1: {MIN, MED, MAX},          // From level 1: normal thresholds
        2: {MIN - BUFFER, MED, MAX}, // From level 2: buffer on MIN
        3: {MIN, MED - BUFFER, MAX}, // From level 3: buffer on MED
        4: {MIN, MED, MAX - BUFFER}, // From level 4: buffer on MAX
    }

    t := thresholds[currentLevel]

    if temp >= t[2] {
        return 4
    } else if temp >= t[1] {
        return 3
    } else if temp >= t[0] {
        return 2
    }
    return 1
}

func setLevel() {
    temperature := common.ReadTemp()
    level := calculateLevel(temperature, current_level)

    if current_level != level {
        log.Printf("Current temperature is %.1f°C (buffer: %.1f°C)\n", temperature, BUFFER)
        setFan(0, level)
        setFan(1, level)
        current_level = level
    }
}

func main() {
    if len(os.Args) < 3 {
        fmt.Fprintf(os.Stderr, "Usage: %s <auto | cpu | disk | all> <level>\n", os.Args[0])
        os.Exit(1)
    }

    if err := rpio.Open(); err != nil {
        log.Fatal(err)
    }
    defer rpio.Close()

    // Install signal handler
    signal_chan := make(chan os.Signal, 1)
    signal.Notify(signal_chan, os.Interrupt, syscall.SIGTERM)

    go func() {
        for {
            s := <-signal_chan
            switch s {
            case os.Interrupt, syscall.SIGTERM:
                log.Print("Exiting...")
                rpio.Close()
                os.Exit(0)
            }
        }
    }()

    var input, err = strconv.ParseUint(os.Args[2], 10, 64)
    if err != nil {
        log.Fatal(err)
    }

    var level = common.Clamp(uint32(input), 0, 4)

    switch os.Args[1] {
    case "cpu":
        setFan(0, level)
    case "disk":
        setFan(1, level)
    case "all":
        setFan(0, level)
        setFan(1, level)
    case "auto":
        log.Print("Starting automatic fan control...")
        for {
            setLevel()
            time.Sleep(time.Second)
        }
    }
}

