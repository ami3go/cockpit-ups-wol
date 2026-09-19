package orchestrator

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/ami3go/cockpit-ups-wol/agent/internal/config"
	"github.com/ami3go/cockpit-ups-wol/agent/internal/host"
	"github.com/ami3go/cockpit-ups-wol/agent/internal/nut"
	"github.com/ami3go/cockpit-ups-wol/agent/internal/policy"
	"github.com/ami3go/cockpit-ups-wol/agent/internal/state"
)

type memoryStore struct{ st state.State; events *[]string }
func(m *memoryStore)Write(s state.State)(state.State,error){s.Sequence=m.st.Sequence+1;m.st=cloneState(s);*m.events=append(*m.events,fmt.Sprintf("write:%s",s.PowerState));for id,hs:=range s.Hosts{if hs.ShutdownState==state.ShutdownRequested{*m.events=append(*m.events,"requested:"+id)}};return cloneState(s),nil}
type fakeProbe struct{ online map[string]bool }
func(p *fakeProbe)Check(_ context.Context,h config.HostConfig)(host.ProbeResult,error){return host.ProbeResult{Known:true,Online:p.online[h.ID]},nil}
func(p *fakeProbe)Wait(_ context.Context,h config.HostConfig,want bool)error{p.online[h.ID]=want;return nil}
type fakeShutdown struct{ events *[]string }
func(s fakeShutdown)Shutdown(_ context.Context,h config.HostConfig)(host.ShutdownResult,error){*s.events=append(*s.events,"shutdown:"+h.ID);return host.ShutdownResult{Disposition:host.ShutdownDirectRequested},nil}
type fakeFSD struct{ events *[]string }
func(f fakeFSD)RequestFSD(_ context.Context,_,_ string)error{*f.events=append(*f.events,"fsd");return nil}
type fakeRecovery struct{ events *[]string; coord *policy.Coordinator }
func(r fakeRecovery)RunNext(_ context.Context,st state.State,hosts []host.Config)(state.State,string,error){*r.events=append(*r.events,"wake");for _,h:=range hosts{hs:=st.Hosts[h.ID];if host.EligibleForRestore(h,hs)&&hs.RecoveryState!=state.RecoveryOnline{hs.RecoveryState=state.RecoveryOnline;st.Hosts[h.ID]=hs;persisted,err:=r.coord.Write(st);return persisted,h.ID,err}};return st,"",nil}

func p(v int)*int{return &v};func s(v string)*string{return &v}
func testConfig()config.Config{return config.Config{Mode:"armed",NUT:config.NUTConfig{Profile:"local-server",UPSName:"ups",Host:"localhost",Port:3493},Outage:config.OutageConfig{GracePeriodSeconds:0},Recovery:config.RecoveryConfig{Enabled:true,UtilityStableSeconds:1,BatteryChargeMin:p(80)},Hosts:[]config.HostConfig{{ID:"pc",Name:"PC",Address:s("pc"),Status:config.StatusConfig{Method:"ping",SuccessConsecutive:1,ProbeIntervalSeconds:1},Shutdown:config.ShutdownConfig{Method:"ssh",Priority:10,TimeoutSeconds:1},Wake:config.WakeConfig{Enabled:true,Priority:10,MaxAttempts:2},RestorePolicy:"previous-state"},{ID:"nas",Name:"NAS",Address:s("nas"),Status:config.StatusConfig{Method:"ping",SuccessConsecutive:1,ProbeIntervalSeconds:1},Shutdown:config.ShutdownConfig{Method:"nut",Priority:20,TimeoutSeconds:1},Wake:config.WakeConfig{Enabled:true,Priority:20,MaxAttempts:2},RestorePolicy:"previous-state"}}}}
func policyConfig(cfg config.Config)policy.Config{charge:=float64(*cfg.Recovery.BatteryChargeMin);crit:=float64(30);return policy.Config{GracePeriod:0,CriticalCharge:&crit,UtilityStable:time.Second,RecoveryChargeMin:&charge}}
func index(events []string,value string)int{for i,e:=range events{if e==value{return i}};return -1}

func TestFullLifecyclePersistsBeforeSideEffects(t *testing.T){
	events:=[]string{};base:=state.New("boot","cfg");base.PowerState=state.Normal;store:=&memoryStore{st:base,events:&events};coord:=policy.NewCoordinator(policy.New(policyConfig(testConfig()),base),store);probe:=&fakeProbe{online:map[string]bool{"pc":true,"nas":true}};ctl:=&Controller{Config:testConfig(),Policy:coord,Probe:probe,Shutdown:fakeShutdown{&events},FSD:fakeFSD{&events},NewTransactionID:func()string{return"outage-1"}};ctl.Recovery=fakeRecovery{events:&events,coord:coord}
	online:=nut.Status{Utility:nut.UtilityOnline};ob:=nut.Status{Utility:nut.UtilityOnBattery};lb:=ob;lb.LowBattery=true
	now:=time.Unix(100,0);if _,err:=ctl.Tick(context.Background(),now,policy.Inputs{UPS:online,NetworkReady:true,HealthSafe:true});err!=nil{t.Fatal(err)}
	if _,err:=ctl.Tick(context.Background(),now.Add(time.Second),policy.Inputs{UPS:ob,NetworkReady:true,HealthSafe:true});err!=nil{t.Fatal(err)}
	if coord.State().Hosts["pc"].WasOnline==nil||!*coord.State().Hosts["pc"].WasOnline{t.Fatal("online snapshot missing")}
	if _,err:=ctl.Tick(context.Background(),now.Add(2*time.Second),policy.Inputs{UPS:lb,NetworkReady:true,HealthSafe:true});err!=nil{t.Fatal(err)}
	if coord.State().PowerState!=state.WaitingForAC{t.Fatalf("expected WAITING_FOR_AC, got %s",coord.State().PowerState)}
	commit:=index(events,"write:SHUTDOWN_COMMITTED");req:=index(events,"requested:pc");sd:=index(events,"shutdown:pc");nutReq:=index(events,"requested:nas");fsd:=index(events,"fsd");if commit<0||req<0||sd<0||nutReq<0||fsd<0||!(commit<req&&req<sd&&nutReq<fsd){t.Fatalf("persistence ordering violated: %#v",events)}
	charge:=85.0;online.ChargePercent=&charge
	if _,err:=ctl.Tick(context.Background(),now.Add(3*time.Second),policy.Inputs{UPS:online,NetworkReady:true,HealthSafe:true});err!=nil{t.Fatal(err)}
	if _,err:=ctl.Tick(context.Background(),now.Add(5*time.Second),policy.Inputs{UPS:online,NetworkReady:true,HealthSafe:true});err!=nil{t.Fatal(err)}
	if _,err:=ctl.Tick(context.Background(),now.Add(6*time.Second),policy.Inputs{UPS:online,NetworkReady:true,HealthSafe:true});err!=nil{t.Fatal(err)}
	if index(events,"wake")<0{t.Fatalf("recovery not invoked: %#v",events)}
	if index(events,"write:RESTORE_HOSTS")>index(events,"wake"){t.Fatalf("wake occurred before RESTORE_HOSTS persisted: %#v",events)}
}

func TestPowerBounceStopsFurtherRecovery(t *testing.T){events:=[]string{};st:=state.New("out","cfg");st.PowerState=state.RestoreHosts;st.ShutdownCommitted=true;st.RecoveryStarted=true;v:=true;st.Hosts["pc"]=state.HostState{WasOnline:&v,RecoveryState:state.RecoveryWaiting};store:=&memoryStore{st:st,events:&events};coord:=policy.NewCoordinator(policy.New(policyConfig(testConfig()),st),store);ctl:=&Controller{Config:testConfig(),Policy:coord,Probe:&fakeProbe{online:map[string]bool{"pc":false}},Shutdown:fakeShutdown{&events},FSD:fakeFSD{&events}};ctl.Recovery=fakeRecovery{events:&events,coord:coord};_,err:=ctl.Tick(context.Background(),time.Now(),policy.Inputs{UPS:nut.Status{Utility:nut.UtilityOnBattery},NetworkReady:true,HealthSafe:true});if err!=nil{t.Fatal(err)};if index(events,"wake")>=0{t.Fatalf("wake occurred during power bounce: %#v",events)};if coord.State().PowerState!=state.OnBattery{t.Fatalf("expected ON_BATTERY, got %s",coord.State().PowerState)}}
