package ipc

import (
	"encoding/json"
	"errors"

	"github.com/ami3go/cockpit-ups-wol/agent/internal/health"
)

type Request struct { ID string `json:"id"`; Method string `json:"method"`; Params json.RawMessage `json:"params,omitempty"` }
type Error struct { Code string `json:"code"`; Message string `json:"message"` }
type Response struct { ID string `json:"id"`; OK bool `json:"ok"`; Result any `json:"result,omitempty"`; Error *Error `json:"error,omitempty"` }

type HealthProvider interface { HealthSnapshot() (health.Snapshot,error) }

type Handler struct { Health HealthProvider }

func (h Handler) Handle(req Request) Response {
	switch req.Method {
	case "GetHealth":
		if h.Health==nil{return failure(req.ID,"DEPENDENCY_UNAVAILABLE","health provider unavailable")}
		snap,err:=h.Health.HealthSnapshot();if err!=nil{return failure(req.ID,"INTERNAL_ERROR",err.Error())}
		return Response{ID:req.ID,OK:true,Result:snap}
	default:
		return failure(req.ID,"UNKNOWN_METHOD","unknown method")
	}
}
func failure(id,code,msg string)Response{return Response{ID:id,OK:false,Error:&Error{Code:code,Message:msg}}}
func DecodeRequest(data []byte)(Request,error){var req Request;if err:=json.Unmarshal(data,&req);err!=nil{return req,err};if req.ID==""||req.Method==""{return req,errors.New("id and method are required")};return req,nil}
