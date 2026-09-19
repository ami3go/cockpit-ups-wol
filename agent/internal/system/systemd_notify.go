package system

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
	"time"
)

func Notify(message string) error {
	socket:=os.Getenv("NOTIFY_SOCKET");if socket==""{return nil};if strings.HasPrefix(socket,"@"){socket="\x00"+socket[1:]}
	addr:=&net.UnixAddr{Name:socket,Net:"unixgram"};conn,err:=net.DialUnix("unixgram",nil,addr);if err!=nil{return fmt.Errorf("systemd notify: %w",err)};defer conn.Close();_,err=conn.Write([]byte(message));return err
}
func Ready()error{return Notify("READY=1")}
func Watchdog()error{return Notify("WATCHDOG=1")}
func Stopping()error{return Notify("STOPPING=1")}
func WatchdogInterval()(time.Duration,bool,error){usec:=os.Getenv("WATCHDOG_USEC");if usec==""{return 0,false,nil};if pidText:=os.Getenv("WATCHDOG_PID");pidText!=""{pid,err:=strconv.Atoi(pidText);if err!=nil{return 0,false,errors.New("invalid WATCHDOG_PID")};if pid!=os.Getpid(){return 0,false,nil}};v,err:=strconv.ParseInt(usec,10,64);if err!=nil||v<=0{return 0,false,errors.New("invalid WATCHDOG_USEC")};return time.Duration(v)*time.Microsecond,true,nil}

// StartWatchdog feeds systemd at half the configured watchdog interval until
// context cancellation. The caller should start it only after runtime
// initialization has succeeded and should cancel it if the event loop stalls.
func StartWatchdog(ctx context.Context) (<-chan error, error) {
	interval,enabled,err:=WatchdogInterval();if err!=nil{return nil,err};ch:=make(chan error,1);if !enabled{close(ch);return ch,nil}
	period:=interval/2;if period<=0{period=time.Millisecond}
	go func(){defer close(ch);ticker:=time.NewTicker(period);defer ticker.Stop();for{select{case<-ctx.Done():return;case<-ticker.C:if err:=Watchdog();err!=nil{ch<-err;return}}}}()
	return ch,nil
}
