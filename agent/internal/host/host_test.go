package host

import (
	"context"
	"errors"
	"net"
	"reflect"
	"testing"

	"github.com/ami3go/cockpit-ups-wol/agent/internal/config"
)

type recordingRunner struct {
	name string
	args []string
	err  error
}
func (r *recordingRunner) Run(_ context.Context, name string, args ...string) ([]byte,error){r.name=name;r.args=append([]string(nil),args...);return nil,r.err}

func strp(v string)*string{return &v}
func intp(v int)*int{return &v}

func TestSSHShutdownUsesFixedArgumentVector(t *testing.T){
	r:=&recordingRunner{}
	h:=config.HostConfig{Address:strp("server.lan"),Shutdown:config.ShutdownConfig{Method:"ssh",TimeoutSeconds:10,SSHUser:strp("powerctl"),SSHKeyFile:strp("/etc/cockpit-ups-wol/secrets/server")}}
	res,err:= (ShutdownExecutor{Runner:r}).Shutdown(context.Background(),h);if err!=nil{t.Fatal(err)}
	if res.Disposition!=ShutdownDirectRequested{t.Fatalf("unexpected disposition %s",res.Disposition)}
	want:=[]string{"-o","BatchMode=yes","-o","StrictHostKeyChecking=yes","-o","ConnectTimeout=10","-i","/etc/cockpit-ups-wol/secrets/server","powerctl@server.lan","sudo","-n","/sbin/shutdown","-h","now"}
	if r.name!="ssh"||!reflect.DeepEqual(r.args,want){t.Fatalf("unexpected exec: %s %#v",r.name,r.args)}
}

func TestSSHRejectsOptionLikeAddress(t *testing.T){
	r:=&recordingRunner{}
	h:=config.HostConfig{Address:strp("-oProxyCommand=bad"),Shutdown:config.ShutdownConfig{Method:"ssh",TimeoutSeconds:10,SSHUser:strp("powerctl"),SSHKeyFile:strp("/safe/key")}}
	if _,err:=(ShutdownExecutor{Runner:r}).Shutdown(context.Background(),h);err==nil{t.Fatal("expected unsafe address rejection")}
	if r.name!=""{t.Fatal("runner should not have been invoked")}
}

func TestNUTAndCommandShutdownSemantics(t *testing.T){
	res,err:=(ShutdownExecutor{}).Shutdown(context.Background(),config.HostConfig{Shutdown:config.ShutdownConfig{Method:"nut"}});if err!=nil||res.Disposition!=ShutdownManagedByNUT{t.Fatalf("NUT result=%+v err=%v",res,err)}
	if _,err:=(ShutdownExecutor{}).Shutdown(context.Background(),config.HostConfig{Shutdown:config.ShutdownConfig{Method:"command"}});err==nil{t.Fatal("command shutdown must fail closed")}
}

func TestTCPStatus(t *testing.T){
	ln,err:=net.Listen("tcp","127.0.0.1:0");if err!=nil{t.Fatal(err)};defer ln.Close()
	port:=ln.Addr().(*net.TCPAddr).Port
	res,err:=(StatusChecker{}).Check(context.Background(),"127.0.0.1",config.StatusConfig{Method:"tcp",Port:intp(port),TimeoutMS:500});if err!=nil{t.Fatal(err)}
	if !res.Known||!res.Online{t.Fatalf("expected online, got %+v",res)}
}

func TestArmedCapabilitiesRejectUnsupportedCommandAndARP(t *testing.T){
	cfg:=config.Config{Hosts:[]config.HostConfig{{ID:"x",Status:config.StatusConfig{Method:"tcp",Port:intp(22)},Shutdown:config.ShutdownConfig{Method:"command"}}}}
	if err:=ValidateArmedCapabilities(cfg);err==nil{t.Fatal("expected command rejection")}
	cfg.Hosts[0].Shutdown.Method="none";cfg.Hosts[0].Status.Method="arp"
	if err:=ValidateArmedCapabilities(cfg);err==nil{t.Fatal("expected arp rejection")}
}

func TestPingRunnerInfrastructureErrorIsReported(t *testing.T){
	r:=&recordingRunner{err:errors.New("ping missing")}
	_,err:=(StatusChecker{Runner:r}).Check(context.Background(),"host.lan",config.StatusConfig{Method:"ping",TimeoutMS:500})
	if err==nil{t.Fatal("expected infrastructure error")}
}
