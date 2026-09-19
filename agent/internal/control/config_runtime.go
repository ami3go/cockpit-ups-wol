package control

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"time"

	"github.com/ami3go/cockpit-ups-wol/agent/internal/health"
	"github.com/ami3go/cockpit-ups-wol/agent/internal/ipc"
)

type CommandRunner interface { Run(context.Context,string,...string)([]byte,error) }
type ExecRunner struct{}
func (ExecRunner) Run(ctx context.Context,name string,args ...string)([]byte,error){return exec.CommandContext(ctx,name,args...).CombinedOutput()}

type ServiceRuntime struct { SocketPath string; Service string; Runner CommandRunner; HealthTimeout time.Duration }
func (r *ServiceRuntime) defaults(){if r.SocketPath==""{r.SocketPath="/run/cockpit-ups-wol/agent.sock"};if r.Service==""{r.Service="cockpit-ups-wol-agent.service"};if r.Runner==nil{r.Runner=ExecRunner{}};if r.HealthTimeout<=0{r.HealthTimeout=20*time.Second}}
func (r *ServiceRuntime) Apply(ctx context.Context,_ string)error{r.defaults();out,err:=r.Runner.Run(ctx,"systemctl","restart",r.Service);if err!=nil{return fmt.Errorf("restart %s: %w: %s",r.Service,err,string(out))};return nil}
func (r *ServiceRuntime) Healthy(ctx context.Context)error{
	r.defaults();deadline:=time.Now().Add(r.HealthTimeout);var last error
	for time.Now().Before(deadline){callCtx,cancel:=context.WithTimeout(ctx,time.Second);resp,err:=ipc.Call(callCtx,r.SocketPath,ipc.Request{ID:"rollback-health",Method:"GetHealth"});cancel();if err==nil&&resp.OK{b,_:=json.Marshal(resp.Result);var snap health.Snapshot;if json.Unmarshal(b,&snap)==nil&&snap.State!=health.FailedSafe{return nil};last=errors.New("agent reported FAILED_SAFE or invalid health snapshot")}else if err!=nil{last=err}else if resp.Error!=nil{last=errors.New(resp.Error.Message)};select{case<-ctx.Done():return ctx.Err();case<-time.After(250*time.Millisecond):}}
	if last==nil{last=errors.New("health IPC did not become ready")};return last
}
