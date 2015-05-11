package resolver

import (
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"strconv"
	"strings"

	"github.com/emicklei/go-restful"
	"github.com/mesosphere/mesos-dns/logging"
	"github.com/mesosphere/mesos-dns/plugins"
	"github.com/mesosphere/mesos-dns/records"
	"github.com/mesosphere/mesos-dns/util"
)

type httpResolverInterface interface {
	records() *records.RecordSet
	getVersion() string
}

// HTTP API resolver plugin
type APIPlugin struct {
	httpi  httpResolverInterface
	ws     *restful.WebService
	config *records.Config
	done   chan struct{}
}

func NewAPIPlugin(httpi httpResolverInterface) *APIPlugin {
	api := &APIPlugin{
		httpi: httpi,
		done:  make(chan struct{}),
	}
	// webserver + available routes
	ws := new(restful.WebService)
	ws.Route(ws.GET("/v1/version").To(api.RestVersion))
	ws.Route(ws.GET("/v1/config").To(api.RestConfig))
	ws.Route(ws.GET("/v1/hosts/{host}").To(api.RestHost))
	ws.Route(ws.GET("/v1/hosts/{host}/ports").To(api.RestPorts))
	ws.Route(ws.GET("/v1/services/{service}").To(api.RestService))
	api.ws = ws
	return api
}

// starts an http server for mesos-dns queries, returns immediately
func (api *APIPlugin) Start(ctx plugins.InitialContext) <-chan error {
	defer util.HandleCrash()

	ctx.RegisterWS(api.ws)
	api.config = ctx.Config()

	portString := strconv.Itoa(api.config.HttpPort)
	errCh := make(chan error, 1)
	go func() {
		select {
		case <-api.done:
			panic("plugin already terminated")
		default:
			defer close(api.done)
		}

		var err error
		defer func() { errCh <- err }()

		var address string
		if api.config.Listener != "" {
			address = net.JoinHostPort(api.config.Listener, portString)
		} else {
			address = ":" + portString
		}
		if err = http.ListenAndServe(address, nil); err != nil {
			err = fmt.Errorf("Failed to setup http server: %v", err)
		} else {
			logging.Error.Println("Not serving http requests any more.")
		}
	}()
	return errCh
}

func (api *APIPlugin) Stop() {
	//TODO(jdef)
}

func (api *APIPlugin) Done() <-chan struct{} {
	return api.done
}

// Reports configuration through REST interface
func (api *APIPlugin) RestConfig(req *restful.Request, resp *restful.Response) {
	output, err := json.Marshal(api.config)
	if err != nil {
		logging.Error.Println(err)
	}

	io.WriteString(resp, string(output))
}

// Reports Mesos-DNS version through REST interface
func (api *APIPlugin) RestVersion(req *restful.Request, resp *restful.Response) {
	mapV := map[string]string{"Service": "Mesos-DNS",
		"Version": api.httpi.getVersion(),
		"URL":     "https://github.com/mesosphere/mesos-dns"}
	output, err := json.Marshal(mapV)
	if err != nil {
		logging.Error.Println(err)
	}
	io.WriteString(resp, string(output))
}

// Reports Mesos-DNS version through http interface
func (api *APIPlugin) RestHost(req *restful.Request, resp *restful.Response) {

	host := req.PathParameter("host")
	// clean up host name
	dom := strings.ToLower(cleanWild(host))
	if dom[len(dom)-1] != '.' {
		dom += "."
	}

	mapH := make([]map[string]string, 0)
	rs := api.httpi.records()

	for _, ip := range rs.As[dom] {
		t := map[string]string{"host": dom, "ip": ip}
		mapH = append(mapH, t)
	}
	empty := (len(rs.As[dom]) == 0)
	if empty {
		t := map[string]string{"host": "", "ip": ""}
		mapH = append(mapH, t)
	}

	output, err := json.Marshal(mapH)
	if err != nil {
		logging.Error.Println(err)
	}
	io.WriteString(resp, string(output))

	// stats
	mesosrq := strings.HasSuffix(dom, api.config.Domain+".")
	if mesosrq {
		logging.CurLog.MesosRequests.Inc()
		if empty {
			logging.CurLog.MesosNXDomain.Inc()
		} else {
			logging.CurLog.MesosSuccess.Inc()
		}
	} else {
		logging.CurLog.NonMesosRequests.Inc()
		logging.CurLog.NonMesosFailed.Inc()
	}

}

// Reports Mesos-DNS version through http interface
func (api *APIPlugin) RestPorts(req *restful.Request, resp *restful.Response) {
	io.WriteString(resp, "To be implemented...")
}

// Reports Mesos-DNS version through http interface
func (api *APIPlugin) RestService(req *restful.Request, resp *restful.Response) {

	var ip string

	service := req.PathParameter("service")

	// clean up service name
	dom := strings.ToLower(cleanWild(service))
	if dom[len(dom)-1] != '.' {
		dom += "."
	}

	mapS := make([]map[string]string, 0)
	rs := api.httpi.records()

	for _, srv := range rs.SRVs[dom] {
		h, port, _ := net.SplitHostPort(srv)
		p, _ := strconv.Atoi(port)
		if len(rs.As[h]) != 0 {
			ip = rs.As[h][0]
		} else {
			ip = ""
		}

		t := map[string]string{"service": service, "host": h, "ip": ip, "port": strconv.Itoa(p)}
		mapS = append(mapS, t)
	}

	empty := (len(rs.SRVs[dom]) == 0)
	if empty {
		t := map[string]string{"service": "", "host": "", "ip": "", "port": ""}
		mapS = append(mapS, t)
	}

	output, err := json.Marshal(mapS)
	if err != nil {
		logging.Error.Println(err)
	}
	io.WriteString(resp, string(output))

	// stats
	mesosrq := strings.HasSuffix(dom, api.config.Domain+".")
	if mesosrq {
		logging.CurLog.MesosRequests.Inc()
		if empty {
			logging.CurLog.MesosNXDomain.Inc()
		} else {
			logging.CurLog.MesosSuccess.Inc()
		}
	} else {
		logging.CurLog.NonMesosRequests.Inc()
		logging.CurLog.NonMesosFailed.Inc()
	}

}
