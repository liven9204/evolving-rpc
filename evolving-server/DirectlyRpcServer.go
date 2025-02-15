package evolving_server

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/yuhao-jack/evolving-rpc/contents"
	"github.com/yuhao-jack/evolving-rpc/model"
	"github.com/yuhao-jack/go-toolx/fun"
	"github.com/yuhao-jack/go-toolx/netx"
	"reflect"
	"strings"
)

type DirectlyRpcServerConfig struct {
	model.EvolvingServerConf
}

type DirectlyRpcServer struct {
	serviceMap     map[string]*service
	evolvingServer *EvolvingServer
}

func NewDirectlyRpcServer(config *DirectlyRpcServerConfig) *DirectlyRpcServer {
	return &DirectlyRpcServer{evolvingServer: NewEvolvingServer(&config.EvolvingServerConf), serviceMap: map[string]*service{}}
}

func (d *DirectlyRpcServer) Register(rcvr any) error {
	s := new(service)
	s.typ = reflect.TypeOf(rcvr)
	s.rcvr = reflect.ValueOf(rcvr)
	s.name = reflect.Indirect(s.rcvr).Type().Name()

	if fun.IsBlank(s.name) {
		return errors.New("no service name for type " + s.typ.String())
	}
	s.method = make(map[string]*methodType)
	buildMethodMap(s)
	if len(s.method) == 0 {
		return errors.New(s.name + " has no exported methods of suitable type")
	}
	d.serviceMap[s.name] = s
	return nil
}
func (d *DirectlyRpcServer) Run() {
	for n, server := range d.serviceMap {
		for s := range server.method {
			d.evolvingServer.SetCommand(fmt.Sprint(n, ".", s), func(dataPack *netx.DataPack, reply netx.IMessage) {
				splitArr := strings.Split(string(reply.GetCommand()), ".")
				var reqv reflect.Value
				ts := d.serviceMap[splitArr[0]]
				tm := ts.method[splitArr[1]]
				reqv = reflect.New(tm.ReqType)

				var err error
				var command = string(reply.GetProtoc())
				switch command {
				case contents.Json:
					err = json.Unmarshal(reply.GetBody(), reqv.Interface())
				default:
					err = unknownProtocErr
				}
				if err != nil {
					reply.SetBody([]byte(err.Error()))
					d.evolvingServer.Execute(dataPack, reply, nil)
					return
				}

				res := tm.method.Func.Call([]reflect.Value{ts.rcvr, reflect.Indirect(reqv)})[0].Interface()
				if res != nil {
					var bytes []byte
					switch command {
					case contents.Json:
						bytes, err = json.Marshal(res)
					default:
						err = unknownProtocErr
					}

					if err != nil {
						reply.SetBody([]byte(err.Error()))
					} else {
						reply.SetBody(bytes)
					}
				}
				d.evolvingServer.Execute(dataPack, reply, nil)
			})
		}
	}
	d.evolvingServer.Start()
}
