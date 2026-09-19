package config

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func validJSON() []byte { return []byte(`{
  "config_version":1,"mode":"dry-run",
  "nut":{"profile":"local-server","ups_name":"ups","host":"localhost","port":3493,"driver":null,"driver_port":null,"power_cycle_capability":"POWER_CYCLE_UNVERIFIED","network":{"mode":"trusted-lan","listen_ipv4":true,"listen_ipv6":false,"allowed_clients":[]},"synology_compatibility":{"enabled":false,"username":"monuser","password":"secret"},"hosts_sync_seconds":60,"final_delay_seconds":15},
  "outage":{"grace_period_seconds":120,"max_on_battery_seconds":null,"critical_battery_percent":30,"critical_runtime_seconds":600,"communication_loss_grace_seconds":30},
  "controller":{"require_ups_backed_power":true,"require_auto_power_on":true},
  "recovery":{"enabled":true,"utility_stable_seconds":120,"battery_charge_min":80,"runtime_min_seconds":null,"recharge_time_seconds":null,"network_wait_seconds":300},
  "health":{"enabled":true,"interval_seconds":60,"probation_seconds":60,"autofix":true,"max_repair_attempts":5},
  "network_dependencies":[],
  "hosts":[]
}`) }
func TestParseStrictValid(t *testing.T){cfg,err:=Parse(validJSON());if err!=nil{t.Fatal(err)};if cfg.Mode!="dry-run"||cfg.NUT.UPSName!="ups"{t.Fatalf("cfg=%+v",cfg)}}
func TestParseUnknownFieldRejected(t *testing.T){b:=strings.Replace(string(validJSON()),`"config_version":1`,`"config_version":1,"mystery":true`,1);if _,err:=Parse([]byte(b));err==nil{t.Fatal("expected unknown-field error")}}
func TestValidationRejectsUnsafeController(t *testing.T){cfg,err:=Parse(validJSON());if err!=nil{t.Fatal(err)};cfg.Controller.RequireUPSBackedPower=false;if err:=Validate(cfg);err==nil{t.Fatal("expected safety error")}}
type fakeRuntime struct{applyErr error;healthCalls int;failHealthAt int}
func(f *fakeRuntime)Apply(context.Context,string)error{return f.applyErr}
func(f *fakeRuntime)Healthy(context.Context)error{f.healthCalls++;if f.failHealthAt>0&&f.healthCalls==f.failHealthAt{return errors.New("unhealthy")};return nil}
func newManager(t *testing.T)*Manager{root:=t.TempDir();now:=time.Date(2026,9,19,10,0,0,0,time.UTC);m:=&Manager{HistoryDir:filepath.Join(root,"history"),ActiveConfigPath:filepath.Join(root,"etc","config.yaml"),Probation:2*time.Second,HealthInterval:time.Second};m.Now=func()time.Time{return now};m.Sleep=func(_ context.Context,d time.Duration)error{now=now.Add(d);return nil};return m}
func TestCandidatePromotesOnlyAfterProbation(t *testing.T){m:=newManager(t);rev,err:=m.Begin("test",validJSON());if err!=nil{t.Fatal(err)};if lkg,_:=m.LastKnownGood();lkg!=""{t.Fatalf("candidate prematurely LKG=%s",lkg)};rt:=&fakeRuntime{};out,err:=m.ActivateAndValidate(context.Background(),rev.RevisionID,rt);if err!=nil{t.Fatal(err)};if out.Status!=RevisionKnownGood{t.Fatalf("status=%s",out.Status)};lkg,_:=m.LastKnownGood();if lkg!=rev.RevisionID{t.Fatalf("lkg=%s",lkg)};if rt.healthCalls<2{t.Fatalf("health calls=%d",rt.healthCalls)}}
func TestFailedCandidateRollsBackToLKG(t *testing.T){m:=newManager(t);first,err:=m.Begin("test",validJSON());if err!=nil{t.Fatal(err)};if _,err:=m.ActivateAndValidate(context.Background(),first.RevisionID,&fakeRuntime{});err!=nil{t.Fatal(err)};changed:=strings.Replace(string(validJSON()),`"mode":"dry-run"`,`"mode":"monitor"`,1);second,err:=m.Begin("test",[]byte(changed));if err!=nil{t.Fatal(err)};_,err=m.ActivateAndValidate(context.Background(),second.RevisionID,&fakeRuntime{failHealthAt:1});if err==nil{t.Fatal("expected rollback error")};active,_:=m.ActiveRevision();if active!=first.RevisionID{t.Fatalf("active=%s want %s",active,first.RevisionID)};manifest,err:=m.Manifest(second.RevisionID);if err!=nil{t.Fatal(err)};if manifest.Status!=RevisionRolledBack{t.Fatalf("status=%s",manifest.Status)};b,err:=os.ReadFile(m.ActiveConfigPath);if err!=nil{t.Fatal(err)};cfg,err:=Parse(b);if err!=nil{t.Fatal(err)};if cfg.Mode!="dry-run"{t.Fatalf("mode=%s",cfg.Mode)}}
func TestInterruptedValidatingRollsBackOnBoot(t *testing.T){m:=newManager(t);first,err:=m.Begin("test",validJSON());if err!=nil{t.Fatal(err)};if _,err:=m.ActivateAndValidate(context.Background(),first.RevisionID,&fakeRuntime{});err!=nil{t.Fatal(err)};changed:=strings.Replace(string(validJSON()),`"mode":"dry-run"`,`"mode":"monitor"`,1);second,err:=m.Begin("test",[]byte(changed));if err!=nil{t.Fatal(err)};content,_:=os.ReadFile(filepath.Join(m.revisionDir(second.RevisionID),"config.yaml"));if err:=writeAtomic(m.ActiveConfigPath,content,0o600);err!=nil{t.Fatal(err)};if err:=m.writePointer("active",second.RevisionID);err!=nil{t.Fatal(err)};mf,_:=m.readManifest(second.RevisionID);mf.Status=RevisionValidating;if err:=m.writeManifest(mf);err!=nil{t.Fatal(err)};if err:=m.RecoverInterrupted(context.Background(),&fakeRuntime{});err!=nil{t.Fatal(err)};active,_:=m.ActiveRevision();if active!=first.RevisionID{t.Fatalf("active=%s want LKG",active)}}
