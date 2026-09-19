package system

import (
	"os"
	"testing"
	"time"
)

func TestWatchdogInterval(t *testing.T){
	oldU,oldP:=os.Getenv("WATCHDOG_USEC"),os.Getenv("WATCHDOG_PID");defer os.Setenv("WATCHDOG_USEC",oldU);defer os.Setenv("WATCHDOG_PID",oldP)
	os.Setenv("WATCHDOG_USEC","1000000");os.Setenv("WATCHDOG_PID","")
	d,ok,err:=WatchdogInterval();if err!=nil||!ok||d!=time.Second{t.Fatalf("d=%v ok=%v err=%v",d,ok,err)}
}
