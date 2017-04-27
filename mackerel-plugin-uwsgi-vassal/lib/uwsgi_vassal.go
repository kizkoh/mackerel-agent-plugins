package mpuwsgivassal

import (
	"encoding/json"
	"errors"
	"flag"
	"net"
	"net/http"
	"strings"

	mp "github.com/mackerelio/go-mackerel-plugin"
)

// UWSGIVassalPlugin mackerel plugin for uWSGI
type UWSGIVassalPlugin struct {
	Socket      string
	Prefix      string
	LabelPrefix string
}

// {
//   "version":"2.0.7-debian",
//   "listen_queue":0,
//   "listen_queue_errors":0,
//   "signal_queue":0,
//   "load":0,
//   "pid":77393,
//   "uid":0,
//   "gid":0,
//   "cwd":"/etc/uwsgi/vassals",
//   "workers": [{
//     "id": 1,
//     "pid": 31759,
//     "requests": 0,
//     "exceptions": 0,
//     "status": "idle",
//     "rss": 0,
//     "vsz": 0,
//     "running_time": 0,
//     "last_spawn": 1317235041,
//     "respawn_count": 1,
//     "tx": 0,
//     "avg_rt": 0,
//     "apps": [{
//       "id": 0,
//       "modifier1": 0,
//       "mountpoint": "",
//       "requests": 0,
//       "exceptions": 0,
//       "chdir": ""
//     }]
//   }, {
//     "id": 2,
//     "pid": 31760,
//     "requests": 0,
//     "exceptions": 0,
//     "status": "idle",
//     "rss": 0,
//     "vsz": 0,
//     "running_time": 0,
//     "last_spawn": 1317235041,
//     "respawn_count": 1,
//     "tx": 0,
//     "avg_rt": 0,
//     "apps": [{
//       "id": 0,
//       "modifier1": 0,
//       "mountpoint": "",
//       "requests": 0,
//       "exceptions": 0,
//       "chdir": ""
//     }]
//   }, {
//     "id": 3,
//     "pid": 31761,
//     "requests": 0,
//     "exceptions": 0,
//     "status": "idle",
//     "rss": 0,
//     "vsz": 0,
//     "running_time": 0,
//     "last_spawn": 1317235041,
//     "respawn_count": 1,
//     "tx": 0,
//     "avg_rt": 0,
//     "apps": [{
//       "id": 0,
//       "modifier1": 0,
//       "mountpoint": "",
//       "requests": 0,
//       "exceptions": 0,
//       "chdir": ""
//     }]
//   }, {
//     "id": 4,
//     "pid": 31762,
//     "requests": 0,
//     "exceptions": 0,
//     "status": "idle",
//     "rss": 0,
//     "vsz": 0,
//     "running_time": 0,
//     "last_spawn": 1317235041,
//     "respawn_count": 1,
//     "tx": 0,
//     "avg_rt": 0,
//     "apps": [{
//       "id": 0,
//       "modifier1": 0,
//       "mountpoint": "",
//       "requests": 0,
//       "exceptions": 0,
//       "chdir": ""
//     }]
//   }
// }

// field types vary between versions

// UWSGIWorker struct
type UWSGIWorker struct {
	Requests uint64 `json:"requests"`
	Status   string `json:"status"`
}

// UWSGIStats sturct for json struct
type UWSGIStats struct {
	ListenQueue uint64        `json:"listen_queue"`
	Workers     []UWSGIWorker `json:"workers"`
}

// FetchMetrics interface for mackerelplugin
func (p UWSGIVassalPlugin) FetchMetrics() (map[string]float64, error) {
	stat := make(map[string]float64)
	stat["busy"] = 0.0
	stat["idle"] = 0.0
	stat["cheap"] = 0.0
	stat["pause"] = 0.0
	stat["requests"] = 0.0

	var decoder *json.Decoder
	if strings.HasPrefix(p.Socket, "unix://") {
		conn, err := net.Dial("unix", strings.TrimPrefix(p.Socket, "unix://"))
		if err != nil {
			return nil, err
		}
		defer conn.Close()
		decoder = json.NewDecoder(conn)
	} else if strings.HasPrefix(p.Socket, "http://") {
		resp, err := http.Get(p.Socket)
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()
		decoder = json.NewDecoder(resp.Body)
	} else {
		err := errors.New("'--socket' is neither http endpoint nor the unix domain socket, try '--help' for more information")
		return nil, err
	}

	var uwsgiStats UWSGIStats
	if err := decoder.Decode(&uwsgiStats); err != nil {
		return nil, err
	}
	stat["queue"] = float64(uwsgiStats.ListenQueue)
	for _, worker := range uwsgiStats.Workers {
		switch worker.Status {
		case "idle", "busy", "cheap", "pause":
			stat[worker.Status]++
		}
		stat["requests"] += float64(worker.Requests)
	}

	return stat, nil
}

// GraphDefinition interface for mackerelplugin
func (p UWSGIVassalPlugin) GraphDefinition() map[string]mp.Graphs {
	labelPrefix := strings.Title(p.Prefix)

	var graphdef = map[string]mp.Graphs{
		(p.Prefix + ".queue"): {
			Label: labelPrefix + " Queue",
			Unit:  "integer",
			Metrics: []mp.Metrics{
				{Name: "queue", Label: "Requests", Diff: false},
			},
		},
		(p.Prefix + ".workers"): {
			Label: labelPrefix + " Workers",
			Unit:  "integer",
			Metrics: []mp.Metrics{
				{Name: "busy", Label: "Busy", Diff: false, Stacked: true},
				{Name: "idle", Label: "Idle", Diff: false, Stacked: true},
				{Name: "cheap", Label: "Cheap", Diff: false, Stacked: true},
				{Name: "pause", Label: "Pause", Diff: false, Stacked: true},
			},
		},
		(p.Prefix + ".req"): {
			Label: labelPrefix + " Requests",
			Unit:  "float",
			Metrics: []mp.Metrics{
				{Name: "requests", Label: "Requests", Diff: true},
			},
		},
	}

	return graphdef
}

// MetricKeyPrefix interface for PluginWithPrefix
func (p UWSGIVassalPlugin) MetricKeyPrefix() string {
	if p.Prefix == "" {
		p.Prefix = "uWSGI"
	}
	return p.Prefix
}

// Do the plugin
func Do() {
	optSocket := flag.String("socket", "", "Socket (must be with prefix of 'http://' or 'unix://')")
	optPrefix := flag.String("metric-key-prefix", "uWSGI", "Prefix")
	optTempfile := flag.String("tempfile", "", "Temp file name")
	flag.Parse()

	uwsgi := UWSGIVassalPlugin{Socket: *optSocket, Prefix: *optPrefix}
	uwsgi.LabelPrefix = strings.Title(uwsgi.Prefix)

	helper := mp.NewMackerelPlugin(uwsgi)
	helper.Tempfile = *optTempfile
	helper.Run()
}
