package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"time"

	"github.com/ami3go/cockpit-ups-wol/agent/internal/config"
	"github.com/ami3go/cockpit-ups-wol/agent/internal/control"
	"github.com/ami3go/cockpit-ups-wol/agent/internal/ipc"
	"github.com/ami3go/cockpit-ups-wol/agent/internal/report"
	"github.com/ami3go/cockpit-ups-wol/agent/internal/version"
)

func main(){
	fs:=flag.NewFlagSet("cockpit-ups-wolctl",flag.ContinueOnError);fs.SetOutput(os.Stderr);showVersion:=fs.Bool("version",false,"print version information and exit");socketPath:=fs.String("socket","/run/cockpit-ups-wol/agent.sock","agent Unix socket");configPath:=fs.String("config","/etc/cockpit-ups-wol/config.yaml","active configuration file");historyDir:=fs.String("history-dir","/var/lib/cockpit-ups-wol/config-history","configuration revision history directory")
	fs.Usage=func(){fmt.Fprintf(fs.Output(),"Usage: cockpit-ups-wolctl [options] <command> [args]\n\nOptions:\n");fs.PrintDefaults();fmt.Fprintf(fs.Output(),"\nCommands:\n  health\n  config-status\n  config-rollback <revision>\n  plan\n  logs\n  config-bootstrap\n")};if err:=fs.Parse(os.Args[1:]);err!=nil{os.Exit(2)};if *showVersion{b,_:=json.Marshal(version.Current());fmt.Println(string(b));return};if fs.NArg()==0{fs.Usage();os.Exit(2)};manager:=&config.Manager{HistoryDir:*historyDir,ActiveConfigPath:*configPath}
	switch fs.Arg(0){
	case"health":ctx,cancel:=context.WithTimeout(context.Background(),5*time.Second);defer cancel();resp,err:=ipc.Call(ctx,*socketPath,ipc.Request{ID:"cli-health",Method:"GetHealth",Params:json.RawMessage(`{}`)});if err!=nil{fatal(err)};printJSON(resp);if !resp.OK{os.Exit(1)}
	case"config-status":status,err:=report.ReadConfigStatus(manager);if err!=nil{fatal(err)};printJSON(status)
	case"config-rollback":if os.Geteuid()!=0{fatal(fmt.Errorf("config-rollback requires root privileges"))};if fs.NArg()!=2{fatal(fmt.Errorf("config-rollback requires exactly one revision id"))};ctx,cancel:=context.WithTimeout(context.Background(),45*time.Second);defer cancel();rt:=&control.ServiceRuntime{SocketPath:*socketPath};if err:=manager.Rollback(ctx,fs.Arg(1),rt);err!=nil{fatal(err)};status,err:=report.ReadConfigStatus(manager);if err!=nil{fatal(err)};printJSON(status)
	case"plan":content,err:=os.ReadFile(*configPath);if err!=nil{fatal(err)};cfg,err:=config.Parse(content);if err!=nil{fatal(err)};printJSON(report.BuildPlan(cfg))
	case"logs":ctx,cancel:=context.WithTimeout(context.Background(),5*time.Second);defer cancel();out,err:=exec.CommandContext(ctx,"journalctl","--no-pager","-u","cockpit-ups-wol-agent.service","-n","100","-o","short-iso").CombinedOutput();if err!=nil{fmt.Fprintln(os.Stderr,"cockpit-ups-wolctl:",err);fmt.Fprint(os.Stderr,string(out));os.Exit(1)};fmt.Print(string(out))
	case"config-bootstrap":content,err:=os.ReadFile(*configPath);if err!=nil{fatal(err)};manifest,err:=manager.BootstrapKnownGood("installer",content);if err!=nil{fatal(err)};printJSON(manifest)
	default:fmt.Fprintf(os.Stderr,"cockpit-ups-wolctl: unknown command %q\n",fs.Arg(0));os.Exit(2)}
}
func fatal(err error){fmt.Fprintln(os.Stderr,"cockpit-ups-wolctl:",err);os.Exit(1)}
func printJSON(v any){b,err:=json.MarshalIndent(v,"","  ");if err!=nil{fatal(err)};fmt.Println(string(b))}
